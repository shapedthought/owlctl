/*
Copyright © 2022 Ed Howard exfhoward@protonmail.com
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/shapedthought/owlctl/config"
	"github.com/shapedthought/owlctl/utils"
	"github.com/spf13/cobra"
)

var (
	targetFlag   string
	instanceFlag string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "owlctl",
	Short: "A CLI application for Veeam APIs",
	Long:  `A CLI application that works with all Veeam APIs`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// --instance: load config, resolve, and activate the instance
		effectiveInstance := instanceFlag
		if effectiveInstance == "" {
			settings, err := utils.TryReadSettings()
			if err != nil {
				return fmt.Errorf("failed to read settings.json: %w (fix or remove the file, then retry)", err)
			}
			effectiveInstance = settings.DefaultInstance
		}

		if instanceFlag != "" && targetFlag != "" {
			return fmt.Errorf("cannot use --instance and --target together")
		}
		if effectiveInstance != "" && targetFlag != "" {
			return fmt.Errorf("cannot use --target when a default instance is set — run 'owlctl instance unset' or use --instance explicitly")
		}

		if effectiveInstance != "" {
			cfg, err := config.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load owlctl.yaml: %w", err)
			}

			resolved, err := config.ResolveInstance(cfg, effectiveInstance)
			if err != nil {
				if instanceFlag == "" {
					// came from DefaultInstance — give a helpful hint
					return fmt.Errorf("default instance %q not found in owlctl.yaml — run 'owlctl instance unset' to clear", effectiveInstance)
				}
				return fmt.Errorf("--instance %q: %w", effectiveInstance, err)
			}

			if err := config.ActivateInstance(resolved); err != nil {
				return fmt.Errorf("failed to activate instance %q: %w", effectiveInstance, err)
			}
			return nil
		}

		// --target (deprecated): only sets OWLCTL_URL
		if targetFlag != "" {
			fmt.Fprintln(os.Stderr, "Warning: --target is deprecated. Use --instance instead.")
			fmt.Fprintln(os.Stderr, "         --instance supports product type, credentials, and per-instance token caching.")

			cfg, err := config.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load owlctl.yaml: %w", err)
			}

			target, err := cfg.GetTarget(targetFlag)
			if err != nil {
				return fmt.Errorf("--target %q: %w", targetFlag, err)
			}

			if err := os.Setenv("OWLCTL_URL", target.URL); err != nil {
				return fmt.Errorf("failed to set OWLCTL_URL: %w", err)
			}
			return nil
		}

		return nil
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&instanceFlag, "instance", "", "Named instance from owlctl.yaml (sets URL, credentials, product)")
	rootCmd.PersistentFlags().StringVar(&targetFlag, "target", "", "Named VBR target from owlctl.yaml (deprecated, use --instance)")
}
