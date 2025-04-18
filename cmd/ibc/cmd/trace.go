package cmd

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func traceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trace [chain-id] [tx-hash]",
		Short: "Trace IBC packets",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			logWriter.AddExtraLogger(func(entry string) {
				cmd.Println(entry)
			})

			network, err := cfg.ToNetwork(ctx, logger, extraGwei)
			if err != nil {
				return errors.Wrap(err, "failed to build network")
			}

			chain, err := network.GetChain(args[0])
			if err != nil {
				return errors.Wrapf(err, "failed to get chain %s", args[0])
			}

			packets, err := chain.GetPackets(ctx, args[1])
			if err != nil {
				return errors.Wrap(err, "failed to get packets")
			}

			if len(packets) > 1 {
				return errors.New("more than one packet found")
			}

			fmt.Printf("Packets: %+v\n", packets)

			return nil
		},
	}

	return cmd
}
