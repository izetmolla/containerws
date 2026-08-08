package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/softwares/seed"
	"github.com/izetmolla/containerws/modules/softwares/service"
	"github.com/spf13/cobra"
)

func init() {
	softwareCmd.AddCommand(softwareStatusCmd)
	softwareCmd.AddCommand(softwareStartCmd)
	softwareCmd.AddCommand(softwareStopCmd)
	softwareCmd.AddCommand(softwareRestartCmd)

	softwareStatusCmd.Flags().Bool("json", false, "print JSON instead of a table")
}

var softwareStatusCmd = &cobra.Command{
	Use:   "status <id|name>",
	Short: "Show systemd status for a software's services",
	Args:  cobra.ExactArgs(1),
	RunE: withStore(func(cmd *cobra.Command, args []string, appClients *config.AppClients) error {
		return runSoftwareServiceAction(cmd, appClients, args[0], "")
	}),
}

var softwareStartCmd = &cobra.Command{
	Use:   "start <id|name>",
	Short: "Start a software's systemd services",
	Args:  cobra.ExactArgs(1),
	RunE: withStore(func(cmd *cobra.Command, args []string, appClients *config.AppClients) error {
		return runSoftwareServiceAction(cmd, appClients, args[0], "start")
	}),
}

var softwareStopCmd = &cobra.Command{
	Use:   "stop <id|name>",
	Short: "Stop a software's systemd services",
	Args:  cobra.ExactArgs(1),
	RunE: withStore(func(cmd *cobra.Command, args []string, appClients *config.AppClients) error {
		return runSoftwareServiceAction(cmd, appClients, args[0], "stop")
	}),
}

var softwareRestartCmd = &cobra.Command{
	Use:   "restart <id|name>",
	Short: "Restart a software's systemd services",
	Args:  cobra.ExactArgs(1),
	RunE: withStore(func(cmd *cobra.Command, args []string, appClients *config.AppClients) error {
		return runSoftwareServiceAction(cmd, appClients, args[0], "restart")
	}),
}

func runSoftwareServiceAction(cmd *cobra.Command, appClients *config.AppClients, key, action string) error {
	seed.SeedIfEmpty(appClients)

	sw, err := findSoftware(appClients.DB(), key)
	if err != nil {
		return err
	}

	units := []string(sw.ServiceUnits)
	if len(units) == 0 {
		return fmt.Errorf("%q has no managed systemd services", sw.Name)
	}

	var st service.Status
	if action == "" {
		st = service.ProbeUnits(units)
	} else {
		st, err = service.ControlUnits(action, units)
		if err != nil {
			printSoftwareServiceStatus(sw.Name, st)
			return err
		}
	}

	asJSON, _ := cmd.Flags().GetBool("json")
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"software_id": sw.ID,
			"name":        sw.Name,
			"status":      st,
		})
	}

	printSoftwareServiceStatus(sw.Name, st)
	return nil
}

func printSoftwareServiceStatus(name string, st service.Status) {
	fmt.Printf("%s — %s\n", name, st.Overall)
	if !st.Managed {
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "UNIT\tACTIVE\tSUB\tNOTES")
	for _, u := range st.Units {
		notes := u.Description
		if u.Error != "" {
			notes = u.Error
		}
		if notes == "" {
			notes = "-"
		}
		sub := u.Sub
		if sub == "" {
			sub = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", u.Unit, u.Active, sub, notes)
	}
	_ = w.Flush()
}
