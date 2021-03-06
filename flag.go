// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

/*
	Package main.flags implements command-line flag parsing.

	Usage

	```shell
	go-gitter [options] "<dir/url>" "<outputDir>" "<answers>"
	```

	## Description

	Use a template to initialize a new project. A template can be a local directory
	or a zip from a URL. Zip files will be downloaded and extracted to a local
	directory.

	## Options

	**--tmplPath**, **-t** URL to a zip or a local path to a directory

	**--answers**, **-a** Path to an answer file.

	**--help**, **-h** Output this documentation.

	**--verbosity** Control the level of information/feedback the program will
	output to the user.

	**--version**, **-v** Output version information.

*/
package main

import (
	"flag"
	"fmt"
)

type cliFlag struct {
	name        string
	short       string
	description string
	valueType   string
}

type cliFlags []cliFlag

var appFlags = cliFlags{
	cliFlag{"tmplPath", "t", "URL to a zip or a local path to a directory.", "string"},
	cliFlag{"appPath", "p", "Path to output the new project.", "string"},
	cliFlag{"answers", "a", "Path to an answer file.", "string"},
	cliFlag{"verbosity", "", "extra detail processing info.", "int"},
	cliFlag{"help", "h", "print usage information.", "bool"},
	cliFlag{"version", "v", "print build version information and exit 0.", "bool"},
}

type flagStorage struct {
	Flags *flag.FlagSet
	ints  map[string]*int
	bools map[string]*bool
	strs  map[string]*string
}

// GetInt Get a flag parsed as an integer.
func (fs *flagStorage) GetInt(key string) (val int, err error) {
	v, ok := fs.ints[key]

	if !ok {
		err = fmt.Errorf("there is no defined int flag %q", key)
	}

	val = *v

	return
}

// GetBool Get a flag parsed as an boolean.
func (fs *flagStorage) GetBool(key string) (val bool, err error) {
	v, ok := fs.bools[key]

	if !ok {
		err = fmt.Errorf("there is no defined int flag %q", key)
		return
	}

	val = *v

	return
}

// GetString Get a flag parsed as an string.
func (fs *flagStorage) GetString(key string) (val string, err error) {
	v, ok := fs.strs[key]
	if !ok {
		err = fmt.Errorf("there is no defined int flag %q", key)
		return
	}

	val = *v

	return
}

// Process any program flags fed into the program and return an unparsed flag-set.
func defineFlags(programName string, handling flag.ErrorHandling) (flagStore *flagStorage, err error) {
	flags := flag.NewFlagSet(programName, handling)
	ints := map[string]*int{}
	bools := map[string]*bool{}
	strs := map[string]*string{}

	for _, f := range appFlags {
		switch f.valueType {
		default:
			strs[f.name] = flags.String(f.name, "", f.description)
			if len(f.short) == 1 {
				flags.StringVar(strs[f.name], f.short, *strs[f.name], f.description)
			}
		case "bool":
			bools[f.name] = flags.Bool(f.name, false, f.description)
			if len(f.short) == 1 {
				flags.BoolVar(bools[f.name], f.short, *bools[f.name], f.description)
			}
		case "int":
			ints[f.name] = flags.Int(f.name, 0, f.description)
			if len(f.short) == 1 {
				flags.IntVar(ints[f.name], f.short, *ints[f.name], f.description)
			}
		}
	}

	flagStore = &flagStorage{
		Flags: flags,
		ints:  ints,
		bools: bools,
		strs:  strs,
	}

	return
}
