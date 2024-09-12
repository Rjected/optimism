package genesis

import (
	"errors"
	"fmt"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum-optimism/optimism/op-service/retry"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/urfave/cli/v2"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	"github.com/ethereum-optimism/optimism/op-service/jsonutil"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	l1RPCFlag = &cli.StringFlag{
		Name:     "l1-rpc",
		Usage:    "RPC URL for an Ethereum L1 node",
		Required: true,
	}
	deployConfigFlag = &cli.PathFlag{
		Name:     "deploy-config",
		Usage:    "Path to deploy config file",
		Required: true,
	}
	l1DeploymentsFlag = &cli.PathFlag{
		Name:     "l1-deployments",
		Usage:    "Path to L1 deployments JSON file as in superchain-registry",
		Required: true,
	}
	outfileL2Flag = &cli.PathFlag{
		Name:  "outfile.l2",
		Usage: "Path to L2 genesis output file",
	}
	outfileRollupFlag = &cli.PathFlag{
		Name:  "outfile.rollup",
		Usage: "Path to rollup output file",
	}

	l1AllocsFlag = &cli.StringFlag{
		Name:  "l1-allocs",
		Usage: "Path to L1 genesis state dump",
	}
	outfileL1Flag = &cli.StringFlag{
		Name:  "outfile.l1",
		Usage: "Path to L1 genesis output file",
	}
	l2AllocsFlag = &cli.StringFlag{
		Name:  "l2-allocs",
		Usage: "Path to L2 genesis state dump",
	}

	l1Flags = []cli.Flag{
		deployConfigFlag,
		l1AllocsFlag,
		l1DeploymentsFlag,
		outfileL1Flag,
	}

	l2Flags = []cli.Flag{
		l1RPCFlag,
		deployConfigFlag,
		l2AllocsFlag,
		l1DeploymentsFlag,
		outfileL2Flag,
		outfileRollupFlag,
	}
)

var Subcommands = cli.Commands{
	{
		Name:  "l1",
		Usage: "Generates a L1 genesis state file",
		Flags: l1Flags,
		Action: func(ctx *cli.Context) error {
			deployConfig := ctx.String(deployConfigFlag.Name)
			config, err := genesis.NewDeployConfig(deployConfig)
			if err != nil {
				return err
			}

			var deployments *genesis.L1Deployments
			if l1Deployments := ctx.String(l1DeploymentsFlag.Name); l1Deployments != "" {
				deployments, err = genesis.NewL1Deployments(l1Deployments)
				if err != nil {
					return err
				}
			}

			if deployments != nil {
				config.SetDeployments(deployments)
			}

			cfg := oplog.DefaultCLIConfig()
			logger := oplog.NewLogger(ctx.App.Writer, cfg)
			if err := config.Check(logger); err != nil {
				return fmt.Errorf("deploy config at %s invalid: %w", deployConfig, err)
			}

			// Check the addresses after setting the deployments
			if err := config.CheckAddresses(); err != nil {
				return fmt.Errorf("deploy config at %s invalid: %w", deployConfig, err)
			}

			var dump *foundry.ForgeAllocs
			if l1Allocs := ctx.String(l1AllocsFlag.Name); l1Allocs != "" {
				dump, err = foundry.LoadForgeAllocs(l1Allocs)
				if err != nil {
					return err
				}
			}

			l1Genesis, err := genesis.BuildL1DeveloperGenesis(config, dump, deployments)
			if err != nil {
				return err
			}

			return jsonutil.WriteJSON(l1Genesis, ioutil.ToStdOutOrFileOrNoop(ctx.String(outfileL1Flag.Name), 0o666))
		},
	},
	{
		Name:  "l2",
		Usage: "Generates an L2 genesis file and rollup config suitable for a deployed network",
		Description: "Generating the L2 genesis depends on knowledge of L1 contract addresses for the bridge to be secure. " +
			"A deploy config and either a deployment directory or an L1 deployments file are used to create the L2 genesis. " +
			"The deploy directory and L1 deployments file are generated by the L1 contract deployments. " +
			"An L1 starting block is necessary, it can either be fetched dynamically using config in the deploy config " +
			"or it can be provided as a JSON file.",
		Flags: l2Flags,
		Action: func(ctx *cli.Context) error {
			cfg := oplog.DefaultCLIConfig()
			logger := oplog.NewLogger(ctx.App.Writer, cfg)

			deployConfig := ctx.Path(deployConfigFlag.Name)
			logger.Info("Deploy config", "path", deployConfig)
			config, err := genesis.NewDeployConfig(deployConfig)
			if err != nil {
				return err
			}

			l1Deployments := ctx.Path(l1DeploymentsFlag.Name)
			l1RPC := ctx.String(l1RPCFlag.Name)

			deployments, err := genesis.NewL1Deployments(l1Deployments)
			if err != nil {
				return fmt.Errorf("cannot read L1 deployments at %s: %w", l1Deployments, err)
			}
			config.SetDeployments(deployments)

			var l2Allocs *foundry.ForgeAllocs
			if l2AllocsPath := ctx.String(l2AllocsFlag.Name); l2AllocsPath != "" {
				l2Allocs, err = foundry.LoadForgeAllocs(l2AllocsPath)
				if err != nil {
					return err
				}
			} else {
				return errors.New("missing l2-allocs")
			}

			// Retrieve SystemConfig.startBlock()
			client, err := ethclient.Dial(l1RPC)
			if err != nil {
				return fmt.Errorf("cannot dial %s: %w", l1RPC, err)
			}

			caller := batching.NewMultiCaller(client.Client(), batching.DefaultBatchSize)
			sysCfg := NewSystemConfigContract(caller, config.SystemConfigProxy)
			startBlock, err := sysCfg.StartBlock(ctx.Context)
			if err != nil {
				return fmt.Errorf("failed to fetch startBlock from SystemConfig: %w", err)
			}

			logger.Info("Using L1 Start Block", "number", startBlock)
			// retry because local devnet can experience a race condition where L1 geth isn't ready yet
			l1StartBlock, err := retry.Do(ctx.Context, 24, retry.Fixed(1*time.Second), func() (*types.Block, error) { return client.BlockByNumber(ctx.Context, startBlock) })
			if err != nil {
				return fmt.Errorf("fetching start block by number: %w", err)
			}
			logger.Info("Fetched L1 Start Block", "hash", l1StartBlock.Hash().Hex())

			// Sanity check the config. Do this after filling in the L1StartingBlockTag
			// if it is not defined.
			if err := config.Check(logger); err != nil {
				return err
			}

			// Build the L2 genesis block
			l2Genesis, err := genesis.BuildL2Genesis(config, l2Allocs, l1StartBlock)
			if err != nil {
				return fmt.Errorf("error creating l2 genesis: %w", err)
			}

			l2GenesisBlock := l2Genesis.ToBlock()
			rollupConfig, err := config.RollupConfig(l1StartBlock, l2GenesisBlock.Hash(), l2GenesisBlock.Number().Uint64())
			if err != nil {
				return err
			}
			if err := rollupConfig.Check(); err != nil {
				return fmt.Errorf("generated rollup config does not pass validation: %w", err)
			}

			if err := jsonutil.WriteJSON(l2Genesis, ioutil.ToAtomicFile(ctx.String(outfileL2Flag.Name), 0o666)); err != nil {
				return err
			}
			return jsonutil.WriteJSON(rollupConfig, ioutil.ToAtomicFile(ctx.String(outfileRollupFlag.Name), 0o666))
		},
	},
}
