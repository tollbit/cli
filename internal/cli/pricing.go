package cli

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tollbit/tollbit-cli/internal/app"
	"github.com/tollbit/tollbit-cli/internal/client/tollbit"
	"github.com/tollbit/tollbit-cli/internal/credentials/agenttoken"
	"github.com/tollbit/tollbit-cli/internal/tokens/agent"
)

type pricingOptions struct {
	userAgent string
	asJSON    bool
}

func newPricingCommand(factory app.Factory) *cobra.Command {
	var opts pricingOptions

	cmd := &cobra.Command{
		Use:   "pricing <url-1>,<url-2>,...,<url-n>",
		Short: "Fetch licensing rates for article URLs",
		Long:  "Fetch licensing rates for one or more article URLs on the TollBit network.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return UsageError("pricing requires exactly one comma-separated URL argument")
			}
			if strings.TrimSpace(args[0]) == "" {
				return UsageError("pricing URL argument must not be empty")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPricing(cmd, factory, opts, args[0])
		},
	}

	cmd.Flags().StringVar(&opts.userAgent, "user-agent", "", "user agent for request")
	cmd.Flags().BoolVar(&opts.asJSON, "json", false, "emit raw JSON response")

	return cmd
}

func runPricing(cmd *cobra.Command, factory app.Factory, opts pricingOptions, urlsArg string) error {
	urls, err := validatePricingURLs(urlsArg)
	if err != nil {
		return err
	}

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

	var resp []tollbit.BatchRateResponseV2
	if app.Config().Auth.RetryOnOBORequired {
		resp, err = agenttoken.WithOBORetry(cmd, credentials, identity, func(token agent.Token) ([]tollbit.BatchRateResponseV2, error) {
			return tollbitClient.BatchGetRates(cmd.Context(), urls, token)
		})
	} else {
		token, tokenErr := credentials.GetAgentToken(cmd, identity)
		if tokenErr != nil {
			return RuntimeError(fmt.Errorf("error fetching agent token: %w", tokenErr))
		}
		resp, err = tollbitClient.BatchGetRates(cmd.Context(), urls, token)
	}
	if err != nil {
		return RuntimeError(fmt.Errorf("error fetching rates: %w", err))
	}

	if opts.asJSON {
		return RuntimeError(writeJSON(cmd.OutOrStdout(), resp))
	}
	printPricingResults(cmd.OutOrStdout(), cmd.ErrOrStderr(), resp)
	return nil
}

func validatePricingURLs(urlsArg string) ([]string, error) {
	urls := splitCommaSeparated(urlsArg)
	if len(urls) == 0 {
		return nil, UsageError("pricing requires at least one URL")
	}
	for _, u := range urls {
		if err := validateArticleURL(u); err != nil {
			return nil, UsageError("%s", err.Error())
		}
	}
	return urls, nil
}

func validateArticleURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", raw, err)
	}
	if strings.TrimSpace(parsed.Scheme) == "" || strings.TrimSpace(parsed.Host) == "" {
		return fmt.Errorf("invalid URL %q: scheme and host are required", raw)
	}
	return nil
}

// normalizeArticleURL validates and canonicalizes article URLs for TollBit API calls.
// Trailing slashes on non-root paths are removed so rates, token, and content requests
// use a consistent URL (content GET routing is sensitive to trailing slashes).
func normalizeArticleURL(raw string) (string, error) {
	if err := validateArticleURL(raw); err != nil {
		return "", err
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("invalid URL %q: %w", raw, err)
	}
	if parsed.Path != "/" && strings.HasSuffix(parsed.Path, "/") {
		parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	}
	return parsed.String(), nil
}

func printPricingResults(stdout, stderr io.Writer, resp []tollbit.BatchRateResponseV2) {
	var firstPricedURL string
	for i, item := range resp {
		if i > 0 {
			fmt.Fprintln(stdout)
		}
		fmt.Fprintln(stdout, item.URL)
		if len(item.Rates) == 0 {
			fmt.Fprintln(stdout, "  (no rates)")
			continue
		}
		if firstPricedURL == "" {
			firstPricedURL = item.URL
		}
		for _, rate := range item.Rates {
			display := licenseDisplayInfo(rate.License)
			label := formatPricingLicenseLabel(rate.License, display)
			line := fmt.Sprintf("  %s · %s", formatPriceMicros(rate.Price.PriceMicros, rate.Price.Currency), label)
			if msg := strings.TrimSpace(rate.Error); msg != "" {
				line += " · error: " + msg
			}
			fmt.Fprintln(stdout, line)
			if display.description != "" {
				fmt.Fprintf(stdout, "    %s\n", display.description)
			} else if display.licenseURL != "" {
				fmt.Fprintf(stdout, "    %s\n", display.licenseURL)
			}
		}
	}
	if firstPricedURL != "" {
		printLeadingCommand(stderr, "To fetch content: tollbit content fetch "+firstPricedURL)
	}
}

const (
	licenseTypeOnDemand         = "ON_DEMAND_LICENSE"
	licenseTypeOnDemandFullUse  = "ON_DEMAND_FULL_USE_LICENSE"
)

type licenseDisplay struct {
	label       string
	description string
	licenseURL  string
}

func licenseDisplayInfo(license tollbit.BatchRateLicenseResponse) licenseDisplay {
	switch strings.TrimSpace(license.LicenseType) {
	case licenseTypeOnDemand:
		return licenseDisplay{
			label:       "Summarization",
			description: "Access and use this content to create summaries and citations.",
		}
	case licenseTypeOnDemandFullUse:
		return licenseDisplay{
			label:       "Full Display",
			description: "Display the full article text for a single request.",
		}
	default:
		return licenseDisplay{
			label:      license.LicenseType,
			licenseURL: strings.TrimSpace(license.LicensePath),
		}
	}
}

func formatPricingLicenseLabel(license tollbit.BatchRateLicenseResponse, display licenseDisplay) string {
	label := display.label
	if label == "" {
		return license.LicenseType
	}
	if display.description != "" {
		if licenseType := strings.TrimSpace(license.LicenseType); licenseType != "" {
			return fmt.Sprintf("%s (%s)", label, licenseType)
		}
	}
	return label
}

func formatPriceMicros(micros int64, currency string) string {
	currency = strings.TrimSpace(currency)
	if currency == "" {
		currency = "?"
	}
	amount := float64(micros) / 1_000_000
	s := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", amount), "0"), ".")
	if s == "" || s == "-" {
		s = "0"
	}
	return currency + " " + s
}
