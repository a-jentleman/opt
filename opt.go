// Package opt is a simple, opinionated, options wrapper
// for [pflag] & [cobra].
//
// It aims to be a simpler alternative to viper.
//
// By default, all options can be specified as:
//  1. Default values (lowest precedent)
//  2. Environment variables
//  3. Command-line flags (highest precedent)
//
// All commands have a name, which is used to set default
// values.
//
// The default environment variable name is the option's
// name, but in upper case (see [strings.ToUpper]). To
// disallow an option from being specified as an environment
// variable, override it environment variable name to be ""
// using [EnvName].
//
// The default command-line flag name is the option's
// name. By default, flags are added to the [cobra.Command]'s
// normal [cobra.FlagSet]. Use [FlagIsPersistent] to add it
// to the persistent [cobra.FlagSet] instead. To disallow
// an option from being specified as a command-line flag,
// override its flag name to be "" using [FlagName].
package opt

import (
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type OptType interface {
	string | bool
}

type opt[T OptType] struct {
	once             sync.Once
	cmd              *cobra.Command
	name             string
	usage            string
	envName          string
	flag             *pflag.Flag
	flagV            T
	flagName         string
	flagShorthand    string
	flagIsPersistent bool
	flagIsDir        bool
	flagIsFile       bool
	envParseFunc     func(string) (T, error)
	defaultValue     T
	v                *T
}

func (o *opt[T]) init() {
	o.once.Do(func() {
		if o.flagName != "" && o.flag.Changed {
			*o.v = o.flagV
			return
		}

		if o.envName != "" {
			if strV, ok := os.LookupEnv(o.envName); ok {
				envV, err := o.envParseFunc(strV)
				if err != nil {
					panic(err)
				}
				*o.v = envV
				return
			}
		}

		*o.v = o.defaultValue
	})
}

// String sets up a string option with the specified name
func String(cmd *cobra.Command, v *string, name string, optFuncs ...OptFunc[string]) {
	doVar(
		cmd,
		v,
		name,
		optFuncs,
		func(in string) (out string, err error) { out = in; return },
		func(flagSet *pflag.FlagSet, o *opt[string]) {
			flagSet.StringVarP(&o.flagV, o.flagName, o.flagShorthand, o.defaultValue, o.usage)
		},
	)
}

// Bool sets up a boolean option with the specified name
func Bool(cmd *cobra.Command, v *bool, name string, optFuncs ...OptFunc[bool]) {
	doVar(
		cmd,
		v,
		name,
		optFuncs,
		strconv.ParseBool,
		func(flagSet *pflag.FlagSet, o *opt[bool]) {
			flagSet.BoolVarP(&o.flagV, o.flagName, o.flagShorthand, o.defaultValue, o.usage)
		},
	)
}

func doVar[T OptType](cmd *cobra.Command, v *T, name string, optFuncs []OptFunc[T], envParseFunc func(in string) (out T, err error), flagCreateFunc func(flagSet *pflag.FlagSet, o *opt[T])) {
	if v == nil {
		panic("opt: v cannot be nil")
	}

	ret := opt[T]{
		cmd:          cmd,
		name:         name,
		flagName:     name,
		envName:      strings.ToUpper(name),
		v:            v,
		envParseFunc: envParseFunc,
	}

	for _, opt := range optFuncs {
		opt(&ret)
	}

	if ret.flagName != "" {
		var flagSet *pflag.FlagSet

		if ret.flagIsPersistent {
			flagSet = cmd.PersistentFlags()
		} else {
			flagSet = cmd.Flags()
		}

		flagCreateFunc(flagSet, &ret)
		ret.flag = cmd.Flag(ret.flagName)

		if ret.flagIsDir {
			cobra.MarkFlagDirname(flagSet, ret.flagName)
		}

		if ret.flagIsFile {
			cobra.MarkFlagFilename(flagSet, ret.flagName)
		}
	}

	cobra.OnInitialize(ret.init)
}

type OptFunc[T OptType] func(*opt[T])

// FlagName overrides the default flag name.
// As stated in the package documentation, if the flag name
// is not overriden, it defaults to the option's name.
//
// To disable flag functionlity entirely for this option, override the
// flag name to be "".
func FlagName[T OptType](flagName string) OptFunc[T] {
	return func(o *opt[T]) {
		o.flagName = flagName
	}
}

// FlagShorthand specifies a shorthand that can be used for the
// flag.
//
// Setting it to "" does nothing.
//
// The shorthand is only applicable when the flag is not disabled. See [FlagName]'s
// documentation for how to disable flags.
func FlagShorthand[T OptType](flagShorthand string) OptFunc[T] {
	return func(o *opt[T]) {
		o.flagShorthand = flagShorthand
	}
}

// FlagIsPersistent specifies that the flag should be added
// to the [cobra.Command]'s persistent [pflag.FlagSet].
func FlagIsPersistent[T OptType]() OptFunc[T] {
	return func(o *opt[T]) {
		o.flagIsPersistent = true
	}
}

// IsDirname specifies that the option's value should be
// a directory name. This is only used with shell auto-completion;
// the caller must validate the actual value before use.
func IsDirname() OptFunc[string] {
	return func(o *opt[string]) {
		o.flagIsDir = true
	}
}

// IsFilename specifies that the option's value should be
// a file name. This is only used with shell auto-completion;
// the caller must validate the actual value before use.
func IsFilename() OptFunc[string] {
	return func(o *opt[string]) {
		o.flagIsFile = true
	}
}

// EnvName overrides the default environment variable name.
//
// As stated in the package documentation, if the environment name
// is not overriden, it defaults to an upper-cased version of the option's name.
//
// To disable environment lookup functionlity entirely for this option, override the
// environment name to be "".
func EnvName[T OptType](envName string) OptFunc[T] {
	return func(o *opt[T]) {
		o.envName = envName
	}
}

// Default sets the default value for this option.
func Default[T OptType](v T) OptFunc[T] {
	return func(o *opt[T]) {
		o.defaultValue = v
	}
}
