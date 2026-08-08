package cmd

import (
	"errors"
	"log"
	"os"
	"strings"

	"github.com/izetmolla/containerws/config"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type cobraFunc func(cmd *cobra.Command, args []string) error

// withViperAndStore initializes Viper and AppClients and passes them to the callback.
// Only [withStore] and the root command should call this; other commands use [withStore].
func withViperAndStore(fn func(cmd *cobra.Command, args []string, v *viper.Viper, appclients *config.AppClients) error) cobraFunc {
	return func(cmd *cobra.Command, args []string) error {
		v, err := initViper(cmd)
		if err != nil {
			return err
		}

		applyDatabaseFlag(cmd)

		appclients, err := config.BootApplication()
		if err != nil {
			return err
		}

		return fn(cmd, args, v, appclients)
	}
}

// applyDatabaseFlag honors root --database / -d before BootApplication.
func applyDatabaseFlag(cmd *cobra.Command) {
	if cmd == nil {
		return
	}
	if dbPath, err := cmd.Flags().GetString("database"); err == nil && dbPath != "" {
		_ = os.Setenv("DATABASE_URL", dbPath)
		return
	}
	if root := cmd.Root(); root != nil {
		if dbPath, err := root.PersistentFlags().GetString("database"); err == nil && dbPath != "" {
			_ = os.Setenv("DATABASE_URL", dbPath)
		}
	}
}

func withStore(fn func(cmd *cobra.Command, args []string, appclients *config.AppClients) error) cobraFunc {
	return withViperAndStore(func(cmd *cobra.Command, args []string, _ *viper.Viper, appclients *config.AppClients) error {
		return fn(cmd, args, appclients)
	})
}

// Generate the replacements for all environment variables. This allows to
// use FB_BRANDING_DISABLE_EXTERNAL environment variables, even when the
// option name is branding.disableExternal.
func generateEnvKeyReplacements(cmd *cobra.Command) []string {
	replacements := []string{}

	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		oldName := strings.ToUpper(f.Name)
		newName := strings.ToUpper(lo.SnakeCase(f.Name))
		replacements = append(replacements, oldName, newName)
	})

	return replacements
}

func initViper(cmd *cobra.Command) (*viper.Viper, error) {
	v := viper.New()

	// Get config file from flag
	cfgFile, err := cmd.Flags().GetString("config")
	if err != nil {
		return nil, err
	}

	// Configuration file
	if cfgFile == "" {
		home, err := homedir.Dir()
		if err != nil {
			return nil, err
		}
		v.AddConfigPath(".")
		v.AddConfigPath(home)
		v.AddConfigPath("/etc/filebrowser/")
		v.SetConfigName(".filebrowser")
	} else {
		v.SetConfigFile(cfgFile)
	}

	// Environment variables
	v.SetEnvPrefix("FB")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(generateEnvKeyReplacements(cmd)...))

	// Bind the flags
	err = v.BindPFlags(cmd.Flags())
	if err != nil {
		return nil, err
	}

	// Read in configuration
	if err := v.ReadInConfig(); err != nil {

		if errors.As(err, &viper.ConfigParseError{}) {
			return nil, err
		}
		if viper.GetBool("start") {
			log.Println("No config file used")
		}
	} else {
		if viper.GetBool("start") {
			log.Printf("Using config file: %s", v.ConfigFileUsed())
		}
	}

	// Return Viper
	return v, nil
}
