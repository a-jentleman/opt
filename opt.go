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

func FlagName[T OptType](flagName string) OptFunc[T] {
	if flagName == "" {
		panic("opt: flagName cannot be empty")
	}
	return func(o *opt[T]) {
		o.flagName = flagName
	}
}

func FlagShorthand[T OptType](flagShorthand string) OptFunc[T] {
	if flagShorthand == "" {
		panic("opt: flagShorthand cannot be empty")
	}
	return func(o *opt[T]) {
		o.flagShorthand = flagShorthand
	}
}

func FlagIsPersistent[T OptType]() OptFunc[T] {
	return func(o *opt[T]) {
		o.flagIsPersistent = true
	}
}

func IsDirname() OptFunc[string] {
	return func(o *opt[string]) {
		o.flagIsDir = true
	}
}

func IsFilename() OptFunc[string] {
	return func(o *opt[string]) {
		o.flagIsFile = true
	}
}

func EnvName[T OptType](envName string) OptFunc[T] {
	if envName == "" {
		panic("opt: envName cannot be empty")
	}
	return func(o *opt[T]) {
		o.envName = envName
	}
}

func Default[T OptType](v T) OptFunc[T] {
	return func(o *opt[T]) {
		o.defaultValue = v
	}
}
