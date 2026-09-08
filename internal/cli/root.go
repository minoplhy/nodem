package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/crypto/bcrypt"
	"node_monitor_go/internal/db"
)

var (
	dbPath string
)

// RootCmd builds and returns the root cobra Command.
func RootCmd(getRepo func() (db.Repository, error), runDaemon func(port uint16) error) *cobra.Command {
	root := &cobra.Command{
		Use:   "node_monitor",
		Short: "Multi-tenant dynamic DNS node monitor and failover manager",
	}

	root.SetGlobalNormalizationFunc(func(f *pflag.FlagSet, name string) pflag.NormalizedName {
		return pflag.NormalizedName(strings.ReplaceAll(name, "_", "-"))
	})

	root.PersistentFlags().StringVar(&dbPath, "db", "node_monitor.db", "Path to SQLite database file")

	// 1. Daemon Command
	var daemonPort uint16
	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "Start the monitoring daemon and web server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDaemon(daemonPort)
		},
	}
	daemonCmd.Flags().Uint16VarP(&daemonPort, "port", "p", 8080, "HTTP server listening port")
	root.AddCommand(daemonCmd)

	// 2. Bootstrap Command
	var bootUser, bootPass string
	bootstrapCmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap the initial Admin user",
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := getRepo()
			if err != nil {
				return err
			}
			ctx := context.Background()
			exists, err := repo.UserExists(ctx)
			if err != nil {
				return err
			}
			if exists {
				color.Red("Error: Application already bootstrapped. Users already exist.")
				return nil
			}

			hash, err := bcrypt.GenerateFromPassword([]byte(bootPass), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			if _, err := repo.CreateUser(ctx, bootUser, string(hash), "Admin"); err != nil {
				return err
			}
			color.Green("Admin user bootstrapped successfully!")
			return nil
		},
	}
	bootstrapCmd.Flags().StringVarP(&bootUser, "username", "u", "", "Admin username")
	bootstrapCmd.Flags().StringVarP(&bootPass, "password", "p", "", "Admin password")
	_ = bootstrapCmd.MarkFlagRequired("username")
	_ = bootstrapCmd.MarkFlagRequired("password")
	root.AddCommand(bootstrapCmd)

	// 3. User Command
	userCmd := &cobra.Command{
		Use:   "user",
		Short: "Manage users",
	}

	var userAddName, userAddPass, userAddRole string
	userAddCmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new user",
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := getRepo()
			if err != nil {
				return err
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(userAddPass), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			if _, err := repo.CreateUser(context.Background(), userAddName, string(hash), userAddRole); err != nil {
				return err
			}
			color.Green("User '%s' added successfully with role '%s'!", userAddName, userAddRole)
			return nil
		},
	}
	userAddCmd.Flags().StringVarP(&userAddName, "username", "u", "", "Username")
	userAddCmd.Flags().StringVarP(&userAddPass, "password", "p", "", "Password")
	userAddCmd.Flags().StringVarP(&userAddRole, "role", "r", "Admin", "User role")
	_ = userAddCmd.MarkFlagRequired("username")
	_ = userAddCmd.MarkFlagRequired("password")

	userListCmd := &cobra.Command{
		Use:   "list",
		Short: "List registered users",
		Run: func(cmd *cobra.Command, args []string) {
			bold := color.New(color.Bold).SprintFunc()
			fmt.Println(bold("Registered Users:"))
			fmt.Println("Feature list is available on the Web Interface.")
		},
	}

	var userDelName string
	userDeleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a user",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Delete users via SQLite or Web Admin console.")
		},
	}
	userDeleteCmd.Flags().StringVarP(&userDelName, "username", "u", "", "Username to delete")

	userCmd.AddCommand(userAddCmd, userListCmd, userDeleteCmd)
	root.AddCommand(userCmd)

	// 4. Provider Command
	providerCmd := &cobra.Command{
		Use:   "provider",
		Short: "Manage DNS providers",
	}

	var provTenant, provName, provType, provAPIURL, provToken, provZone string
	provAddCmd := &cobra.Command{
		Use:   "add",
		Short: "Add a DNS provider",
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := getRepo()
			if err != nil {
				return err
			}
			ctx := context.Background()
			user, err := repo.GetUserByUsername(ctx, provTenant)
			if err != nil || user == nil {
				return fmt.Errorf("tenant user '%s' not found", provTenant)
			}

			if _, err := repo.CreateProvider(ctx, user.ID, provName, provType, provAPIURL, provToken, provZone); err != nil {
				return err
			}
			color.Green("DNS Provider '%s' added successfully for user '%s'!", provName, provTenant)
			return nil
		},
	}
	provAddCmd.Flags().StringVar(&provTenant, "tenant-username", "", "Tenant username")
	provAddCmd.Flags().StringVarP(&provName, "name", "n", "", "Provider name")
	provAddCmd.Flags().StringVarP(&provType, "provider-type", "p", "", "Cloudflare or Technitium")
	provAddCmd.Flags().StringVar(&provAPIURL, "api-url", "https://api.cloudflare.com", "API base URL")
	provAddCmd.Flags().StringVarP(&provToken, "token", "t", "", "API token")
	provAddCmd.Flags().StringVarP(&provZone, "zone", "z", "", "DNS zone name/ID")
	_ = provAddCmd.MarkFlagRequired("tenant-username")
	_ = provAddCmd.MarkFlagRequired("name")
	_ = provAddCmd.MarkFlagRequired("provider-type")
	_ = provAddCmd.MarkFlagRequired("token")
	_ = provAddCmd.MarkFlagRequired("zone")

	var provListTenant string
	provListCmd := &cobra.Command{
		Use:   "list",
		Short: "List DNS providers",
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := getRepo()
			if err != nil {
				return err
			}
			ctx := context.Background()
			user, err := repo.GetUserByUsername(ctx, provListTenant)
			if err != nil || user == nil {
				return fmt.Errorf("tenant user '%s' not found", provListTenant)
			}
			list, err := repo.ListProviders(ctx, user.ID)
			if err != nil {
				return err
			}

			bold := color.New(color.Bold).SprintFunc()
			fmt.Println(bold(fmt.Sprintf("DNS Providers for '%s':", provListTenant)))
			for _, p := range list {
				fmt.Printf("- ID: %d, Name: %s, Type: %s, Zone: %s\n", p.ID, p.Name, p.ProviderType, p.Zone)
			}
			return nil
		},
	}
	provListCmd.Flags().StringVar(&provListTenant, "tenant-username", "", "Tenant username")
	_ = provListCmd.MarkFlagRequired("tenant-username")

	var provDelTenant string
	var provDelID int64
	provDeleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a DNS provider",
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := getRepo()
			if err != nil {
				return err
			}
			ctx := context.Background()
			user, err := repo.GetUserByUsername(ctx, provDelTenant)
			if err != nil || user == nil {
				return fmt.Errorf("tenant user '%s' not found", provDelTenant)
			}
			if err := repo.DeleteProvider(ctx, user.ID, provDelID); err != nil {
				return err
			}
			color.Green("DNS Provider ID %d deleted successfully!", provDelID)
			return nil
		},
	}
	provDeleteCmd.Flags().StringVar(&provDelTenant, "tenant-username", "", "Tenant username")
	provDeleteCmd.Flags().Int64VarP(&provDelID, "id", "i", 0, "Provider ID")
	_ = provDeleteCmd.MarkFlagRequired("tenant-username")
	_ = provDeleteCmd.MarkFlagRequired("id")

	providerCmd.AddCommand(provAddCmd, provListCmd, provDeleteCmd)
	root.AddCommand(providerCmd)

	// 5. Group Command
	groupCmd := &cobra.Command{
		Use:   "group",
		Short: "Manage target groups",
	}

	var grpTenant, grpName, grpRecord string
	var grpProviderID, grpInterval int64
	grpAddCmd := &cobra.Command{
		Use:   "add",
		Short: "Add a target group",
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := getRepo()
			if err != nil {
				return err
			}
			ctx := context.Background()
			user, err := repo.GetUserByUsername(ctx, grpTenant)
			if err != nil || user == nil {
				return fmt.Errorf("tenant user '%s' not found", grpTenant)
			}
			if _, err := repo.CreateGroup(ctx, user.ID, grpName, grpRecord, grpProviderID, grpInterval); err != nil {
				return err
			}
			color.Green("Target Group '%s' added successfully for user '%s'!", grpName, grpTenant)
			return nil
		},
	}
	grpAddCmd.Flags().StringVar(&grpTenant, "tenant-username", "", "Tenant username")
	grpAddCmd.Flags().StringVarP(&grpName, "name", "n", "", "Target group name")
	grpAddCmd.Flags().StringVarP(&grpRecord, "dns-record", "d", "", "Target DNS record (e.g. node.example.com)")
	grpAddCmd.Flags().Int64Var(&grpProviderID, "provider-id", 0, "DNS provider ID")
	grpAddCmd.Flags().Int64Var(&grpInterval, "interval", 60, "Check interval in seconds")
	_ = grpAddCmd.MarkFlagRequired("tenant-username")
	_ = grpAddCmd.MarkFlagRequired("name")
	_ = grpAddCmd.MarkFlagRequired("dns-record")
	_ = grpAddCmd.MarkFlagRequired("provider-id")

	var grpListTenant string
	grpListCmd := &cobra.Command{
		Use:   "list",
		Short: "List target groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := getRepo()
			if err != nil {
				return err
			}
			ctx := context.Background()
			user, err := repo.GetUserByUsername(ctx, grpListTenant)
			if err != nil || user == nil {
				return fmt.Errorf("tenant user '%s' not found", grpListTenant)
			}
			list, err := repo.ListGroups(ctx, user.ID)
			if err != nil {
				return err
			}

			bold := color.New(color.Bold).SprintFunc()
			fmt.Println(bold(fmt.Sprintf("Target Groups for '%s':", grpListTenant)))
			for _, g := range list {
				fmt.Printf("- ID: %d, Name: %s, Record: %s, Interval: %ds\n", g.ID, g.Name, g.DnsRecord, g.CheckIntervalSecs)
			}
			return nil
		},
	}
	grpListCmd.Flags().StringVar(&grpListTenant, "tenant-username", "", "Tenant username")
	_ = grpListCmd.MarkFlagRequired("tenant-username")

	var grpDelTenant string
	var grpDelID int64
	grpDeleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a target group",
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := getRepo()
			if err != nil {
				return err
			}
			ctx := context.Background()
			user, err := repo.GetUserByUsername(ctx, grpDelTenant)
			if err != nil || user == nil {
				return fmt.Errorf("tenant user '%s' not found", grpDelTenant)
			}
			if err := repo.DeleteGroup(ctx, user.ID, grpDelID); err != nil {
				return err
			}
			color.Green("Target Group ID %d deleted successfully!", grpDelID)
			return nil
		},
	}
	grpDeleteCmd.Flags().StringVar(&grpDelTenant, "tenant-username", "", "Tenant username")
	grpDeleteCmd.Flags().Int64VarP(&grpDelID, "id", "i", 0, "Group ID")
	_ = grpDeleteCmd.MarkFlagRequired("tenant-username")
	_ = grpDeleteCmd.MarkFlagRequired("id")

	groupCmd.AddCommand(grpAddCmd, grpListCmd, grpDeleteCmd)
	root.AddCommand(groupCmd)

	return root
}

// GetDBPath returns the configured SQLite database file path.
func GetDBPath() string {
	if dbPath == "" {
		return "node_monitor.db"
	}
	return dbPath
}

func Execute() {
	// Wrapper if called directly
	os.Exit(1)
}
