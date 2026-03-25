package main

import (
	"fmt"
	"os"

	"github.com/meganerd/kz-bridge/internal/config"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	cfgFile string
)

func main() {
	root := &cobra.Command{
		Use:     "kz-bridge",
		Short:   "Bridge between Karakeep and Zotero",
		Long:    "Syncs bookmarks from Karakeep to your Zotero library, enriching metadata via a Zotero Translation Server.",
		Version: version,
	}

	root.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ~/.config/kz-bridge/config.yaml)")

	root.AddCommand(syncCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func syncCmd() *cobra.Command {
	var (
		dryRun        bool
		since         string
		collection    string
		includeAITags bool
	)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync bookmarks from Karakeep to Zotero",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			// Override config with flags
			if cmd.Flags().Changed("dry-run") {
				cfg.DryRun = dryRun
			}
			if cmd.Flags().Changed("since") {
				cfg.Since = since
			}
			if cmd.Flags().Changed("collection") {
				cfg.Collection = collection
			}
			if cmd.Flags().Changed("include-ai-tags") {
				cfg.IncludeAITags = includeAITags
			}

			fmt.Printf("kz-bridge sync (dry-run: %v)\n", cfg.DryRun)
			// Bridge orchestration will be wired here in KZ-7
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be synced without writing")
	cmd.Flags().StringVar(&since, "since", "", "only sync bookmarks created after this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&collection, "collection", "", "target Zotero collection key")
	cmd.Flags().BoolVar(&includeAITags, "include-ai-tags", true, "include Karakeep AI-generated tags")

	return cmd
}
