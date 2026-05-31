package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"code/crawler"

	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Name:      "hexlet-go-crawler",
		Usage:     "analyze a website structure",
		ArgsUsage: "",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "depth",
				Usage: "crawl depth",
				Value: 10,
			},
			&cli.IntFlag{
				Name:  "retries",
				Usage: "number of retries for failed requests",
				Value: 1,
			},
			&cli.DurationFlag{
				Name:  "delay",
				Usage: "delay between requests (example: 200ms, 1s)",
				Value: 0,
			},
			&cli.DurationFlag{
				Name:  "timeout",
				Usage: "per-request timeout",
				Value: 15 * time.Second,
			},
			&cli.IntFlag{
				Name:  "rps",
				Usage: "limit requests per second (overrides delay)",
				Value: 0,
			},
			&cli.StringFlag{
				Name:  "user-agent",
				Usage: "custom user agent",
			},
			&cli.IntFlag{
				Name:  "workers",
				Usage: "number of concurrent workers",
				Value: 4,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			url := cmd.Args().First()
			if url == "" {
				if _, err := fmt.Fprintln(os.Stdout, "URL is required"); err != nil {
					return err
				}

				if _, err := fmt.Fprintln(os.Stdout); err != nil {
					return err
				}

				return cli.ShowAppHelp(cmd)
			}

			timeout := cmd.Duration("timeout")

			result, err := crawler.Analyze(ctx, crawler.Options{
				URL:         url,
				Depth:       cmd.Int("depth"),
				Retries:     cmd.Int("retries"),
				Delay:       cmd.Duration("delay"),
				Timeout:     timeout,
				RPS:         cmd.Int("rps"),
				UserAgent:   cmd.String("user-agent"),
				Concurrency: cmd.Int("workers"),
				IndentJSON:  true,
				HTTPClient: &http.Client{
					Timeout: timeout,
				},
			})

			if len(result) > 0 {
				if _, err := os.Stdout.Write(result); err != nil {
					return err
				}

				if _, err := os.Stdout.Write([]byte("\n")); err != nil {
					return err
				}
			}

			return err
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		if _, printErr := fmt.Fprintln(os.Stderr, err); printErr != nil {
			os.Exit(1)
		}

		os.Exit(1)
	}
}
