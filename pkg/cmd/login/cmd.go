package login

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	sdk "github.com/openshift-online/ocm-sdk-go"

	"github.com/skeeey/xcm-cli/pkg/configs"
	"github.com/skeeey/xcm-cli/pkg/constants"
	"github.com/skeeey/xcm-cli/pkg/genericflags"
	"github.com/skeeey/xcm-cli/pkg/helpers"
)

var args struct {
	url      string
	tokenURL string
	token    string
	insecure bool
}

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in",
		Long: "Log in, saving the credentials to the configuration file.\n" +
			"The recommend way is using '--token', which you can obtain at: " +
			constants.OfflineTokenPage,
		Args: cobra.NoArgs,
		RunE: run,
	}

	addFlags(cmd.Flags())
	genericflags.AddFlag(cmd.Flags())

	return cmd
}

func addFlags(flags *pflag.FlagSet) {
	flags.StringVar(
		&args.url,
		"url",
		sdk.DefaultURL,
		"URL of the xCM API gateway.",
	)

	flags.StringVar(
		&args.tokenURL,
		"token-url",
		sdk.DefaultTokenURL,
		"OpenID token URL.",
	)

	flags.StringVar(
		&args.token,
		"token",
		"",
		fmt.Sprintf("Red Hat user API token which you can obtain at '%s'.", constants.OfflineTokenPage),
	)
}

func run(cmd *cobra.Command, argv []string) error {
	if err := helpers.ValidateURL(args.url); err != nil {
		return err
	}

	haveToken := args.token != ""
	if !haveToken {
		return fmt.Errorf("flag '--token' is mandatory")
	}

	// Load the configuration file:
	cfg, err := configs.LoadAPIConfig()
	if err != nil {
		return fmt.Errorf("cannot load config file: %v", err)
	}

	if haveToken {
		// Encrypted tokens are assumed to be refresh tokens:
		if configs.IsEncryptedToken(args.token) {
			cfg.AccessToken = ""
			cfg.RefreshToken = args.token
		} else {
			// If a token has been provided parse it:
			token, err := configs.ParseToken(args.token)
			if err != nil {
				return fmt.Errorf("cannot parse token '%s': %v", args.token, err)
			}
			// Put the token in the place of the configuration that corresponds to its type:
			typ, err := configs.TokenType(token)
			if err != nil {
				return fmt.Errorf("cannot extract type from 'typ' claim of token '%s': %v", args.token, err)
			}
			switch typ {
			case "Bearer", "":
				cfg.AccessToken = args.token
				cfg.RefreshToken = ""
			case "Refresh", "Offline":
				cfg.AccessToken = ""
				cfg.RefreshToken = args.token
			default:
				return fmt.Errorf("unknown token type '%s' in token '%s'", typ, args.token)
			}
		}
	}

	// Update the configuration with the values given in the command line:
	cfg.TokenURL = args.tokenURL
	cfg.Scopes = sdk.DefaultScopes //TODO ??
	cfg.URL = args.url
	cfg.Insecure = args.insecure

	// Create a connection and get the token to verify that the crendentials are correct:
	connection, err := cfg.Connection()
	if err != nil {
		return fmt.Errorf("cannot create connection: %v", err)
	}
	accessToken, refreshToken, err := connection.Tokens()
	if err != nil {
		return fmt.Errorf("cannot get token: %v", err)
	}

	// Save the configuration, but clear the user name and password before unless we have
	// explicitly been asked to store them persistently:
	cfg.AccessToken = accessToken
	cfg.RefreshToken = refreshToken
	err = cfg.Save()
	if err != nil {
		return fmt.Errorf("cannot save config file: %v", err)
	}

	fmt.Fprintln(os.Stdout, "Login successful")
	return nil
}
