// Command kuberexctl holds the commands the kuberex image runs in the pod.
//
//	kuberexctl accounts ACCOUNTS-FILE        print the accounts the file names
//	kuberexctl services SERVICES-FILE        start the services the file lists
//	kuberexctl claude record|forget|resume   keep and resume Claude Code sessions
//	kuberexctl serve --login-server-pid PID --accounts NAMES
//	                                         accept shutdown requests, as root
//	kuberexctl shutdown                      shut the server container down
//	kuberexctl stop-servers                  stop every account's Rex server, as root
package main

import (
	"fmt"
	"os"

	"sarak.sh/mikluko/kuberex/internal/accountsfile"
	"sarak.sh/mikluko/kuberex/internal/claude"
	"sarak.sh/mikluko/kuberex/internal/control"
	"sarak.sh/mikluko/kuberex/internal/services"
)

const usage = "usage: kuberexctl accounts|services|claude|serve|shutdown|stop-servers [ARGS]"

func main() { os.Exit(run(os.Args[1:])) }

// run runs the subcommand args name and returns the process's exit code.
func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, usage)
		return 2
	}
	switch args[0] {
	case "accounts":
		return accountsfile.Main(args[1:])
	case "services":
		return services.Main(args[1:])
	case "claude":
		return claude.Main(args[1:])
	case "serve":
		control.ServeMain(args[1:])
		return 1
	case "shutdown":
		return control.ShutdownMain(args[1:])
	case "stop-servers":
		return control.StopServersMain(args[1:])
	}
	fmt.Fprintln(os.Stderr, usage)
	return 2
}
