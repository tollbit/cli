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
	"github.com/tollbit/tollbit-cli/internal/client/auth"
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

If the configured user agent is not registered for content access, the CLI lists
available user agents to choose from and saves the selection for future fetches.`

const createUserAgentURL = "https://hack.tollbit.com/my-agents"

type fetchOptions struct {
	confirm        bool
	toDisk         string
	rateIndex      int
	userAgentIndex int
	agentName      string
	userAgent      string
	asJSON         bool
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
	cmd.Flags().IntVar(&opts.userAgentIndex, "user-agent-index", 0, "1-based index when selecting a registered user agent")
	cmd.Flags().StringVar(&opts.agentName, "agent-name", "", "agent identity name")
	cmd.Flags().StringVar(&opts.userAgent, "agent-user-agent", "", "registered TollBit user agent for content fetch")
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
		Name:      flagChangedStr(cmd, "agent-name"),
		UserAgent: flagChangedStr(cmd, "agent-user-agent"),
	}
	identity, err := credentials.ResolveIdentity(cmd.Context(), identityOpts)
	if err != nil {
		return RuntimeError(fmt.Errorf("error resolving identity: %w", err))
	}

	var batchResp []tollbit.BatchRateResponseV2
	if app.Config().Auth.RetryOnOBORequired {
		batchResp, err = agenttoken.WithOBORetry(cmd, credentials, identity, func(token agent.Token) ([]tollbit.BatchRateResponseV2, error) {
			return tollbitClient.BatchGetRates(cmd.Context(), []string{articleURL}, token, identity.UserAgent)
		})
	} else {
		token, tokenErr := credentials.GetAgentToken(cmd, identity)
		if tokenErr != nil {
			return RuntimeError(fmt.Errorf("error fetching agent token: %w", tokenErr))
		}
		batchResp, err = tollbitClient.BatchGetRates(cmd.Context(), []string{articleURL}, token, identity.UserAgent)
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

	tokenReq := tollbit.CreateContentAccessTokenRequest{
		URL:            articleURL,
		UserAgent:      identity.UserAgent,
		MaxPriceMicros: selectedRate.Price.PriceMicros,
		Currency:       selectedRate.Price.Currency,
		LicenseType:    selectedRate.License.LicenseType,
		LicenseCuid:    selectedRate.License.Cuid,
		Format:         "markdown",
	}

	contentToken, identity, err := createContentAccessTokenWithUserAgentRetry(
		cmd,
		credentials,
		tollbitClient,
		identity,
		tokenReq,
		opts,
		app.Config().Auth.RetryOnOBORequired,
	)
	if err != nil {
		return RuntimeError(err)
	}

	content, err := tollbitClient.GetContent(cmd.Context(), articleURL, contentToken, identity.UserAgent)
	if err != nil {
		return RuntimeError(fmt.Errorf("error fetching content: %w", err))
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

func createContentAccessTokenWithUserAgentRetry(
	cmd *cobra.Command,
	credentials *agenttoken.CredentialManager,
	tollbitClient tollbit.Client,
	identity auth.AgentIdentity,
	tokenReq tollbit.CreateContentAccessTokenRequest,
	opts fetchOptions,
	retryOnOBO bool,
) (string, auth.AgentIdentity, error) {
	if strings.TrimSpace(identity.UserAgent) == "" {
		var resolveErr error
		identity, resolveErr = resolveRegisteredUserAgent(cmd, credentials, tollbitClient, identity, opts, retryOnOBO)
		if resolveErr != nil {
			return "", identity, resolveErr
		}
		tokenReq.UserAgent = identity.UserAgent
	}

	create := func(token agent.Token) (tollbit.CreateContentAccessTokenResponse, error) {
		return tollbitClient.CreateContentAccessToken(cmd.Context(), tokenReq, token, identity.UserAgent)
	}

	var resp tollbit.CreateContentAccessTokenResponse
	var err error
	if retryOnOBO {
		resp, err = agenttoken.WithOBORetry(cmd, credentials, identity, create)
	} else {
		var token agent.Token
		token, err = credentials.GetAgentToken(cmd, identity)
		if err != nil {
			return "", identity, err
		}
		resp, err = create(token)
	}
	if err == nil {
		return resp.Token, identity, nil
	}
	if !isUserAgentNotRegistered(err) {
		return "", identity, fmt.Errorf("error creating content access token: %w", err)
	}

	identity, err = resolveRegisteredUserAgent(cmd, credentials, tollbitClient, identity, opts, retryOnOBO)
	if err != nil {
		return "", identity, err
	}

	tokenReq.UserAgent = identity.UserAgent
	if retryOnOBO {
		resp, err = agenttoken.WithOBORetry(cmd, credentials, identity, create)
	} else {
		var token agent.Token
		token, err = credentials.GetAgentToken(cmd, identity)
		if err != nil {
			return "", identity, err
		}
		resp, err = create(token)
	}
	if err != nil {
		return "", identity, fmt.Errorf("error creating content access token: %w", err)
	}
	return resp.Token, identity, nil
}

func resolveRegisteredUserAgent(
	cmd *cobra.Command,
	credentials *agenttoken.CredentialManager,
	tollbitClient tollbit.Client,
	identity auth.AgentIdentity,
	opts fetchOptions,
	retryOnOBO bool,
) (auth.AgentIdentity, error) {
	agents, listErr := listUserAgents(cmd, credentials, tollbitClient, identity, retryOnOBO)
	if listErr != nil {
		return identity, listErr
	}
	if len(agents) == 0 {
		return identity, noRegisteredUserAgentsError(cmd.ErrOrStderr())
	}

	selected, selectErr := selectUserAgent(cmd.ErrOrStderr(), cmd.InOrStdin(), agents, opts.userAgentIndex, opts.asJSON)
	if selectErr != nil {
		return identity, selectErr
	}

	identity.UserAgent = selected.UserAgent
	if saveErr := credentials.SaveIdentity(cmd.Context(), identity); saveErr != nil {
		return identity, fmt.Errorf("error saving user agent: %w", saveErr)
	}
	return identity, nil
}

func listUserAgents(
	cmd *cobra.Command,
	credentials *agenttoken.CredentialManager,
	tollbitClient tollbit.Client,
	identity auth.AgentIdentity,
	retryOnOBO bool,
) ([]tollbit.UserAgentResponse, error) {
	if retryOnOBO {
		return agenttoken.WithOBORetry(cmd, credentials, identity, func(token agent.Token) ([]tollbit.UserAgentResponse, error) {
			return tollbitClient.ListUserAgents(cmd.Context(), token)
		})
	}
	token, err := credentials.GetAgentToken(cmd, identity)
	if err != nil {
		return nil, fmt.Errorf("error fetching agent token: %w", err)
	}
	agents, err := tollbitClient.ListUserAgents(cmd.Context(), token)
	if err != nil {
		return nil, fmt.Errorf("error listing user agents: %w", err)
	}
	return agents, nil
}

func selectUserAgent(stderr io.Writer, stdin io.Reader, agents []tollbit.UserAgentResponse, userAgentIndex int, asJSON bool) (tollbit.UserAgentResponse, error) {
	if len(agents) == 1 {
		return agents[0], nil
	}
	if userAgentIndex > 0 {
		if userAgentIndex > len(agents) {
			return tollbit.UserAgentResponse{}, UsageError("user-agent-index %d is out of range (1-%d)", userAgentIndex, len(agents))
		}
		return agents[userAgentIndex-1], nil
	}
	if asJSON {
		return tollbit.UserAgentResponse{}, UsageError("user agent not registered; pass --user-agent-index with --json")
	}
	index, err := promptSelectIndex(stderr, stdin, "registered user agents", len(agents), func(i int) string {
		return agents[i].UserAgent
	})
	if err != nil {
		return tollbit.UserAgentResponse{}, RuntimeError(err)
	}
	return agents[index], nil
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

func isUserAgentNotRegistered(err error) bool {
	var problem problemjson.Problem
	return errors.As(err, &problem) && problem.IsUserAgentNotRegistered()
}

func noRegisteredUserAgentsError(w io.Writer) error {
	fmt.Fprintf(w, "No registered user agents found.\nCreate one at %s, then run this command again.\n", createUserAgentURL)
	return errors.New("no registered user agents available")
}
