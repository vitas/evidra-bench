package main

import "github.com/spf13/pflag"

func registerBenchAPIKeyFlag(flags *pflag.FlagSet, target *string) {
	flags.StringVar(target, "bench-api-key", *target, "Bench API key (env: BENCH_API_KEY)")
	if flag := flags.Lookup("bench-api-key"); flag != nil {
		flag.DefValue = ""
	}
}
