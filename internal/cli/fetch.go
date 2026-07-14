package cli

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tollbit/tollbit-cli/internal/app"
	"github.com/tollbit/tollbit-cli/internal/client/tollbit"
	"github.com/tollbit/tollbit-cli/internal/credentials/agenttoken"
	"github.com/tollbit/tollbit-cli/internal/errorsx/problemjson"
	"github.com/tollbit/tollbit-cli/internal/tokens/agent"
)

const fetchLongHelp = `Fetch licensed publisher content from the TollBit network.

Every fetch charges money. Pricing is shown before the request proceeds unless
you pass --confirm (automation still incurs the listed cost).

Use --toDisk to save fetched content to a file path.

When multiple license rates are returned, you will be prompted to choose one
unless --rate-index is set. With --json and multiple rates, --rate-index is required.

When no user agent is configured, the org default -tbcli- agent is used.
Set one with auth set --user-agent or pass --user-agent on this command.`

type fetchOptions struct {
	confirm   bool
	toDisk    string
	rateIndex int
	userAgent string
	asJSON    bool
}

func newFetchCommand(factory app.Factory) *cobra.Command {
	var opts fetchOptions

	cmd := &cobra.Command{
		Use:   "fetch <url>",
		Short: "Fetch licensed publisher content (paid)",
		Long:  fetchLongHelp,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return UsageError("fetch requires exactly one URL argument")
			}
			if strings.TrimSpace(args[0]) == "" {
				return UsageError("fetch URL argument must not be empty")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFetch(cmd, factory, opts, args[0])
		},
	}

	cmd.Flags().BoolVar(&opts.confirm, "confirm", false, "skip interactive price confirmation (fetch still charges money)")
	cmd.Flags().StringVar(&opts.toDisk, "toDisk", "", "write fetched content to the given file path")
	cmd.Flags().IntVar(&opts.rateIndex, "rate-index", 0, "1-based index when multiple license rates are returned")
	cmd.Flags().StringVar(&opts.userAgent, "user-agent", "", "registered TollBit user agent for content fetch")
	cmd.Flags().BoolVar(&opts.asJSON, "json", false, "emit raw JSON response")

	return cmd
}

func runFetch(cmd *cobra.Command, factory app.Factory, opts fetchOptions, articleURL string) error {
	normalizedURL, err := normalizeArticleURL(articleURL)
	if err != nil {
		return UsageError("%s", err.Error())
	}
	articleURL = normalizedURL

	app, err := appForCommand(factory, cmd)
	if err != nil {
		return RuntimeError(err)
	}
	credentials, err := app.Credentials()
	if err != nil {
		return RuntimeError(err)
	}
	tollbitClient, err := app.Tollbit()
	if err != nil {
		return RuntimeError(err)
	}

	identityOpts := agenttoken.ResolveIdentityOptions{
		UserAgent: flagChangedStr(cmd, "user-agent"),
	}
	identity, err := credentials.ResolveIdentity(cmd.Context(), identityOpts)
	if err != nil {
		return RuntimeError(fmt.Errorf("error resolving identity: %w", err))
	}

	var batchResp []tollbit.BatchRateResponseV2
	if app.Config().Auth.RetryOnOBORequired {
		batchResp, err = agenttoken.WithOBORetry(cmd, credentials, identity, func(token agent.Token) ([]tollbit.BatchRateResponseV2, error) {
			return tollbitClient.BatchGetRates(cmd.Context(), []string{articleURL}, token)
		})
	} else {
		token, tokenErr := credentials.GetAgentToken(cmd, identity)
		if tokenErr != nil {
			return RuntimeError(fmt.Errorf("error fetching agent token: %w", tokenErr))
		}
		batchResp, err = tollbitClient.BatchGetRates(cmd.Context(), []string{articleURL}, token)
	}
	if err != nil {
		return RuntimeError(fmt.Errorf("error fetching rates: %w", err))
	}

	rates := availableRates(batchResp, articleURL)
	if len(rates) == 0 {
		return RuntimeError(errors.New("no license rates available for URL"))
	}

	selectedRate, err := selectRate(cmd.ErrOrStderr(), cmd.InOrStdin(), rates, opts.rateIndex, opts.asJSON)
	if err != nil {
		return err
	}

	if !opts.confirm {
		accepted, promptErr := promptConfirm(
			cmd.ErrOrStderr(),
			cmd.InOrStdin(),
			selectedRate.Price.PriceMicros,
			selectedRate.Price.Currency,
			selectedRate.License,
		)
		if promptErr != nil {
			return RuntimeError(promptErr)
		}
		if !accepted {
			return RuntimeError(errors.New("fetch cancelled"))
		}
	}

	stopSpinner := startSpinner(cmd.ErrOrStderr(), !opts.asJSON, "Fetching")
	defer stopSpinner()

	tokenReq := tollbit.CreateContentAccessTokenRequest{
		URL:            articleURL,
		MaxPriceMicros: selectedRate.Price.PriceMicros,
		Currency:       selectedRate.Price.Currency,
		LicenseType:    selectedRate.License.LicenseType,
		LicenseCuid:    selectedRate.License.Cuid,
		Format:         "markdown",
	}
	if ua := strings.TrimSpace(identity.UserAgent); ua != "" {
		tokenReq.UserAgent = ua
	}

	var contentToken string
	if app.Config().Auth.RetryOnOBORequired {
		resp, tokenErr := agenttoken.WithOBORetry(cmd, credentials, identity, func(token agent.Token) (tollbit.CreateContentAccessTokenResponse, error) {
			return tollbitClient.CreateContentAccessToken(cmd.Context(), tokenReq, token)
		})
		if tokenErr != nil {
			stopSpinner()
			return userAgentFetchError(cmd.ErrOrStderr(), tokenErr, "error creating content access token", app.Config().Agent.RegisterUserAgentURL)
		}
		contentToken = resp.Token
	} else {
		token, tokenErr := credentials.GetAgentToken(cmd, identity)
		if tokenErr != nil {
			stopSpinner()
			return RuntimeError(fmt.Errorf("error fetching agent token: %w", tokenErr))
		}
		resp, tokenErr := tollbitClient.CreateContentAccessToken(cmd.Context(), tokenReq, token)
		if tokenErr != nil {
			stopSpinner()
			return userAgentFetchError(cmd.ErrOrStderr(), tokenErr, "error creating content access token", app.Config().Agent.RegisterUserAgentURL)
		}
		contentToken = resp.Token
	}

	var content tollbit.GetContentResponse
	if app.Config().Auth.RetryOnOBORequired {
		content, err = agenttoken.WithOBORetry(cmd, credentials, identity, func(token agent.Token) (tollbit.GetContentResponse, error) {
			return tollbitClient.GetContent(cmd.Context(), articleURL, contentToken, identity.UserAgent, token)
		})
	} else {
		token, tokenErr := credentials.GetAgentToken(cmd, identity)
		if tokenErr != nil {
			stopSpinner()
			return RuntimeError(fmt.Errorf("error fetching agent token: %w", tokenErr))
		}
		content, err = tollbitClient.GetContent(cmd.Context(), articleURL, contentToken, identity.UserAgent, token)
	}
	stopSpinner()
	if err != nil {
		return userAgentFetchError(cmd.ErrOrStderr(), err, "error fetching content", app.Config().Agent.RegisterUserAgentURL)
	}

	if opts.asJSON {
		if err := writeJSON(cmd.OutOrStdout(), content); err != nil {
			return RuntimeError(err)
		}
	} else {
		if _, err := io.WriteString(cmd.OutOrStdout(), content.Content.Body); err != nil {
			return RuntimeError(err)
		}
		if !strings.HasSuffix(content.Content.Body, "\n") && content.Content.Body != "" {
			if _, err := io.WriteString(cmd.OutOrStdout(), "\n"); err != nil {
				return RuntimeError(err)
			}
		}
	}

	if opts.toDisk != "" {
		if err := writeFetchToDisk(opts.toDisk, content, opts.asJSON); err != nil {
			return RuntimeError(err)
		}
	}

	return nil
}

func availableRates(batch []tollbit.BatchRateResponseV2, articleURL string) []tollbit.BatchDeveloperRateResponse {
	for _, item := range batch {
		if item.URL != articleURL {
			continue
		}
		var rates []tollbit.BatchDeveloperRateResponse
		for _, rate := range item.Rates {
			if strings.TrimSpace(rate.Error) != "" {
				continue
			}
			rates = append(rates, rate)
		}
		return rates
	}
	if len(batch) == 1 {
		var rates []tollbit.BatchDeveloperRateResponse
		for _, rate := range batch[0].Rates {
			if strings.TrimSpace(rate.Error) != "" {
				continue
			}
			rates = append(rates, rate)
		}
		return rates
	}
	return nil
}

func selectRate(stderr io.Writer, stdin io.Reader, rates []tollbit.BatchDeveloperRateResponse, rateIndex int, asJSON bool) (tollbit.BatchDeveloperRateResponse, error) {
	if len(rates) == 1 {
		return rates[0], nil
	}
	if rateIndex > 0 {
		if rateIndex > len(rates) {
			return tollbit.BatchDeveloperRateResponse{}, UsageError("rate-index %d is out of range (1-%d)", rateIndex, len(rates))
		}
		return rates[rateIndex-1], nil
	}
	if asJSON {
		return tollbit.BatchDeveloperRateResponse{}, UsageError("multiple license rates returned; pass --rate-index with --json")
	}
	index, err := promptSelectIndex(stderr, stdin, "license rates", len(rates), func(i int) string {
		rate := rates[i]
		display := licenseDisplayInfo(rate.License)
		label := formatPricingLicenseLabel(rate.License, display)
		return fmt.Sprintf("%s · %s", formatPriceMicros(rate.Price.PriceMicros, rate.Price.Currency), label)
	})
	if err != nil {
		return tollbit.BatchDeveloperRateResponse{}, RuntimeError(err)
	}
	return rates[index], nil
}

func promptSelectIndex(stderr io.Writer, stdin io.Reader, label string, count int, describe func(int) string) (int, error) {
	fmt.Fprintf(stderr, "Select %s:\n", label)
	for i := 0; i < count; i++ {
		fmt.Fprintf(stderr, "  %d) %s\n", i+1, describe(i))
	}
	fmt.Fprint(stderr, "Choice: ")
	scanner := bufio.NewScanner(stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return 0, err
		}
		return 0, errors.New("selection required")
	}
	choice := strings.TrimSpace(scanner.Text())
	var selected int
	if _, err := fmt.Sscanf(choice, "%d", &selected); err != nil || selected < 1 || selected > count {
		return 0, fmt.Errorf("invalid selection %q", choice)
	}
	return selected - 1, nil
}

func promptConfirm(stderr io.Writer, stdin io.Reader, priceMicros int64, currency string, license tollbit.BatchRateLicenseResponse) (bool, error) {
	display := licenseDisplayInfo(license)
	label := formatPricingLicenseLabel(license, display)
	fmt.Fprintf(stderr, "Fetch will cost %s (%s). Proceed? [y/N]: ",
		formatPriceMicros(priceMicros, currency), label)
	scanner := bufio.NewScanner(stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return false, err
		}
		return false, nil
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return answer == "y" || answer == "yes", nil
}

func writeFetchToDisk(path string, content tollbit.GetContentResponse, asJSON bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	var data []byte
	var err error
	if asJSON {
		var buf bytes.Buffer
		if err = writeJSON(&buf, content); err != nil {
			return err
		}
		data = buf.Bytes()
	} else {
		data = []byte(content.Content.Body)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write output file: %w", err)
	}
	return nil
}

func userAgentFetchError(stderr io.Writer, err error, prefix, registerURL string) error {
	if isUserAgentNotRegistered(err) {
		fmt.Fprintln(stderr, err.Error())
		fmt.Fprintf(stderr, "Register a user agent at %s or run auth set --user-agent.\n", registerURL)
		return RuntimeError(fmt.Errorf("%s: %w", prefix, err))
	}
	return RuntimeError(fmt.Errorf("%s: %w", prefix, err))
}

func isUserAgentNotRegistered(err error) bool {
	var problem problemjson.Problem
	return errors.As(err, &problem) && problem.IsUserAgentNotRegistered()
}
