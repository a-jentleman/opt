package opt

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type OptType interface{ string | bool | int }

var pkgSet []interface {
	beforeParse()
	afterParse()
}

func Parse() {
	for _, o := range pkgSet {
		o.beforeParse()
	}

	pflag.Parse()

	for _, o := range pkgSet {
		o.afterParse()
	}
}

type Opt[T OptType] interface {
	Lookup() (v T, ok bool)
	MustLookup() (v T)

	beforeParse()
	afterParse()
}

type opt[T OptType] struct {
	builder Builder[T]
}

func (o opt[T]) MustLookup() (v T) {
	v, ok := o.Lookup()
	if !ok {
		panic("opt: missing expected opt")
	}

	return v
}

func (o opt[T]) Lookup() (v T, ok bool) {
	str, ok := func() (string, bool) {
		if flagName, _, ok := o.builder.getFlag(); ok {
			v := pflag.Lookup(flagName)
			if v != nil && v.Changed {
				return v.Value.String(), true
			}
		}

		if envName, ok := o.builder.getEnv(); ok {
			if envVal, ok := os.LookupEnv(envName); ok {
				return envVal, true
			}
		}

		if defVal, ok := o.builder.getDefault(); ok {
			return defVal, true
		}

		return "", false
	}()

	if !ok {
		var zero T
		return zero, false
	}

	return o.builder.getConvFunc()(str), true
}

func (o opt[T]) beforeParse() {
	o.builder.beforeParse(o.builder)
}

func (o opt[T]) afterParse() {
	o.builder.afterParse(o.builder)
}

type bCommon[T OptType] struct {
	up Builder[T]
}

func (b bCommon[T]) getKey() (key string) {
	if b.up == nil {
		return
	}
	return b.up.getKey()
}

func (b bCommon[T]) getFlag() (flagName, flagShorthand string, ok bool) {
	if b.up == nil {
		return
	}
	return b.up.getFlag()
}

func (b bCommon[T]) getEnv() (envName string, ok bool) {
	if b.up == nil {
		return
	}
	return b.up.getEnv()
}

func (b bCommon[T]) getDefault() (v string, ok bool) {
	if b.up == nil {
		return
	}
	return b.up.getDefault()
}

func (b bCommon[T]) getUsage() (usage string, ok bool) {
	if b.up == nil {
		return
	}
	return b.up.getUsage()
}

func (b bCommon[T]) beforeParse(bd Builder[T]) {
	if b.up == nil {
		return
	}

	b.up.beforeParse(bd)
}

func (b bCommon[T]) afterParse(bd Builder[T]) {
	if b.up == nil {
		return
	}

	b.up.afterParse(bd)
}

func (b bCommon[T]) getConvFunc() (convFunc func(string) T) {
	if b.up == nil {
		return
	}

	return b.up.getConvFunc()
}

type bRoot[T OptType] struct {
	bCommon[T]
	key      string
	convFunc func(string) T
}

func (b bRoot[T]) getKey() string {
	return b.key
}

func (b bRoot[T]) getConvFunc() func(string) T {
	return b.convFunc
}

func (b bRoot[T]) Flag(flagName string, optFuncs ...FlagOptFunc[T]) Builder[T] {
	return newFlagBuilder(b, flagName, "", optFuncs)
}

func (b bRoot[T]) FlagP(flagName string, flagShorthand string, optFuncs ...FlagOptFunc[T]) Builder[T] {
	return newFlagBuilder(b, flagName, flagShorthand, optFuncs)
}

func (b bRoot[T]) Env(envName string) Builder[T] {
	return bEnv[T]{bCommon: bCommon[T]{up: b}, envName: envName}
}

func (b bRoot[T]) Default(v string) Builder[T] {
	return bDefault[T]{bCommon: bCommon[T]{up: b}, def: v}
}

func (b bRoot[T]) Usage(usage string) Builder[T] {
	return bUsage[T]{bCommon: bCommon[T]{up: b}, usage: usage}
}

func (b bRoot[T]) Build() Opt[T] {
	ret := opt[T]{builder: b}
	pkgSet = append(pkgSet, ret)
	return ret
}

type bFlag[T OptType] struct {
	bCommon[T]
	flagName      string
	flagShorthand string
	isDir         bool
	isFile        bool
	isRequired    bool
	v             string
	ok            bool
}

type FlagOptFunc[T OptType] func(b *bFlag[T]) *bFlag[T]

func newFlagBuilder[T OptType](b Builder[T], flagName, flagShorthand string, optFuncs []FlagOptFunc[T]) (ret *bFlag[T]) {
	return applyOpts(&bFlag[T]{bCommon: bCommon[T]{up: b}, flagName: flagName, flagShorthand: flagShorthand}, optFuncs)
}

func (b *bFlag[T]) getFlag() (flagName, flagShorthand string, ok bool) {
	return b.flagName, b.flagShorthand, true
}

func (b *bFlag[T]) Flag(flagName string, optFuncs ...FlagOptFunc[T]) Builder[T] {
	return newFlagBuilder(b, flagName, "", optFuncs)
}

func (b *bFlag[T]) FlagP(flagName string, flagShorthand string, optFuncs ...FlagOptFunc[T]) Builder[T] {
	return newFlagBuilder(b, flagName, flagShorthand, optFuncs)
}

func (b *bFlag[T]) Env(envName string) Builder[T] {
	return bEnv[T]{bCommon: bCommon[T]{up: b}, envName: envName}
}

func (b *bFlag[T]) Default(v string) Builder[T] {
	return bDefault[T]{bCommon: bCommon[T]{up: b}, def: v}
}

func (b *bFlag[T]) Usage(usage string) Builder[T] {
	return bUsage[T]{bCommon: bCommon[T]{up: b}, usage: usage}
}

func (b *bFlag[T]) Build() Opt[T] {
	ret := opt[T]{builder: b}
	pkgSet = append(pkgSet, ret)
	return ret
}

func (b *bFlag[T]) beforeParse(bd Builder[T]) {
	b.bCommon.beforeParse(bd)

	if b.flagName == "" {
		b.flagName = bd.getKey()
	}

	if b.flagName == "" {
		return
	}

	def, _ := bd.getDefault()
	usage, _ := bd.getUsage()
	pflag.StringP(b.flagName, b.flagShorthand, def, usage)

	if b.isDir {
		cobra.MarkFlagDirname(pflag.CommandLine, b.flagName)
	}

	if b.isFile {
		cobra.MarkFlagFilename(pflag.CommandLine, b.flagName)
	}

	if b.isRequired {
		cobra.MarkFlagRequired(pflag.CommandLine, b.flagName)
	}
}

func FlagIsDirName[T OptType]() FlagOptFunc[T] {
	return func(b *bFlag[T]) *bFlag[T] {
		b.isDir = true
		return b
	}
}

func FlagIsFileName[T OptType]() FlagOptFunc[T] {
	return func(b *bFlag[T]) *bFlag[T] {
		b.isFile = true
		return b
	}
}

func FlagIsRequired[T OptType]() FlagOptFunc[T] {
	return func(b *bFlag[T]) *bFlag[T] {
		b.isRequired = true
		return b
	}
}

type bEnv[T OptType] struct {
	bCommon[T]
	envName string
	v       string
	ok      bool
}

func (b bEnv[T]) getEnv() (envName string, ok bool) {
	return b.envName, true
}

func (b bEnv[T]) Flag(flagName string, optFuncs ...FlagOptFunc[T]) Builder[T] {
	return newFlagBuilder(b, flagName, "", optFuncs)
}

func (b bEnv[T]) FlagP(flagName string, flagShorthand string, optFuncs ...FlagOptFunc[T]) Builder[T] {
	return newFlagBuilder(b, flagName, flagShorthand, optFuncs)
}

func (b bEnv[T]) Env(envName string) Builder[T] {
	return bEnv[T]{bCommon: bCommon[T]{up: b}, envName: envName}
}

func (b bEnv[T]) Default(v string) Builder[T] {
	return bDefault[T]{bCommon: bCommon[T]{up: b}, def: v}
}

func (b bEnv[T]) Usage(usage string) Builder[T] {
	return bUsage[T]{bCommon: bCommon[T]{up: b}, usage: usage}
}

func (b bEnv[T]) Build() Opt[T] {
	ret := opt[T]{builder: b}
	pkgSet = append(pkgSet, ret)
	return ret
}

type bDefault[T OptType] struct {
	bCommon[T]
	def string
}

func (b bDefault[T]) getDefault() (def string, ok bool) {
	return b.def, true
}

func (b bDefault[T]) Flag(flagName string, optFuncs ...FlagOptFunc[T]) Builder[T] {
	return newFlagBuilder(b, flagName, "", optFuncs)
}

func (b bDefault[T]) FlagP(flagName string, flagShorthand string, optFuncs ...FlagOptFunc[T]) Builder[T] {
	return newFlagBuilder(b, flagName, flagShorthand, optFuncs)
}

func (b bDefault[T]) Env(envName string) Builder[T] {
	return bEnv[T]{bCommon: bCommon[T]{up: b}, envName: envName}
}

func (b bDefault[T]) Default(v string) Builder[T] {
	return bDefault[T]{bCommon: bCommon[T]{up: b}, def: v}
}

func (b bDefault[T]) Usage(usage string) Builder[T] {
	return bUsage[T]{bCommon: bCommon[T]{up: b}, usage: usage}
}

func (b bDefault[T]) Build() Opt[T] {
	ret := opt[T]{builder: b}
	pkgSet = append(pkgSet, ret)
	return ret
}

type bUsage[T OptType] struct {
	bCommon[T]
	usage string
}

func (b bUsage[T]) getUsage() (usage string, ok bool) {
	return b.usage, true
}

func (b bUsage[T]) Flag(flagName string, optFuncs ...FlagOptFunc[T]) Builder[T] {
	return newFlagBuilder(b, flagName, "", optFuncs)
}

func (b bUsage[T]) FlagP(flagName string, flagShorthand string, optFuncs ...FlagOptFunc[T]) Builder[T] {
	return newFlagBuilder(b, flagName, flagShorthand, optFuncs)
}

func (b bUsage[T]) Env(envName string) Builder[T] {
	return bEnv[T]{bCommon: bCommon[T]{up: b}, envName: envName}
}

func (b bUsage[T]) Default(v string) Builder[T] {
	return bDefault[T]{bCommon: bCommon[T]{up: b}, def: v}
}

func (b bUsage[T]) Usage(usage string) Builder[T] {
	return bUsage[T]{bCommon: bCommon[T]{up: b}, usage: usage}
}

func (b bUsage[T]) Build() Opt[T] {
	ret := opt[T]{builder: b}
	pkgSet = append(pkgSet, ret)
	return ret
}

func applyOpts[T OptType, B Builder[T], OptFunc ~func(B) B](b B, optFuncs []OptFunc) B {
	for _, optFunc := range optFuncs {
		b = optFunc(b)
	}
	return b
}

type Builder[T OptType] interface {
	getKey() (key string)

	Flag(flagName string, optFuncs ...FlagOptFunc[T]) Builder[T]
	FlagP(flagName string, flagShorthand string, optFuncs ...FlagOptFunc[T]) Builder[T]
	getFlag() (flagName, flagShorthand string, ok bool)

	Env(envName string) Builder[T]
	getEnv() (envName string, ok bool)

	Default(v string) Builder[T]
	getDefault() (v string, ok bool)

	Usage(usage string) Builder[T]
	getUsage() (usage string, ok bool)

	getConvFunc() func(string) T
	beforeParse(Builder[T])
	afterParse(Builder[T])

	Build() Opt[T]
}

func BuildString(key string) Builder[string] {
	ret := bRoot[string]{key: key, convFunc: func(in string) string { return in }}
	return ret
}

func BuildBool(key string) Builder[bool] {
	ret := bRoot[bool]{key: key, convFunc: func(in string) bool {
		b, err := strconv.ParseBool(in)
		if err != nil {
			panic(fmt.Errorf("opt: failed to parse %q into bool", in))
		}
		return b
	}}
	return ret
}

func BuildInt(key string) Builder[int] {
	ret := bRoot[int]{key: key, convFunc: func(in string) int {
		i, err := strconv.Atoi(in)
		if err != nil {
			panic(fmt.Errorf("opt: failed to parse %q into int", in))
		}
		return i
	}}
	return ret
}
