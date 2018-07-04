/*
 * Copyright 2018 The Service Manager Authors
 *
 *    Licensed under the Apache License, Version 2.0 (the "License");
 *    you may not use this file except in compliance with the License.
 *    You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *    Unless required by applicable law or agreed to in writing, software
 *    distributed under the License is distributed on an "AS IS" BASIS,
 *    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *    See the License for the specific language governing permissions and
 *    limitations under the License.
 */

// Package env contains logic for working with environment, flags and file configs via Viper
package env

import (
	"fmt"
	"strings"

	"github.com/fatih/structs"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Environment represents an abstraction over the environment from which Service Manager configuration will be loaded
//go:generate counterfeiter . Environment
type Environment interface {
	Load() error
	Get(key string) interface{}
	Set(key string, value interface{})
	Unmarshal(value interface{}) error
}

// ViperEnv implements env.Environment and provides a way to load environment variables and config files via viper
type ViperEnv struct {
	Viper      *viper.Viper
	configFile *ConfigFile
	envPrefix  string
}

// ConfigFile describes the name and the format of the file to be used to load the configuration in the environment
type ConfigFile struct {
	Name   string
	Path   string
	Format string
}

// Default returns the default environment configuration to be loaded from application.yml
func Default(envPrefix string) *ViperEnv {
	configFile := &ConfigFile{
		Path:   ".",
		Name:   "application",
		Format: "yml",
	}
	return New(configFile, envPrefix)
}

// New returns a new application environment loaded from the given configuration file with variables prefixed by the given prefix
func New(file *ConfigFile, envPrefix string) *ViperEnv {
	return &ViperEnv{
		Viper:      viper.New(),
		configFile: file,
		envPrefix:  envPrefix,
	}
}

// Load reads the config provided by the config file into viper,  configures loading from environment variables
// and loading from command line pflags. Also parses the command-line flags from os.Args[1:].
// Must be called after all flags are defined and before environment and flags are accessed by the program.
func (v *ViperEnv) Load() error {
	v.Viper.AddConfigPath(v.configFile.Path)
	v.Viper.SetConfigName(v.configFile.Name)
	v.Viper.SetConfigType(v.configFile.Format)
	v.Viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.Viper.SetEnvPrefix(v.envPrefix)
	v.Viper.AutomaticEnv()

	if err := v.Viper.BindPFlags(pflag.CommandLine); err != nil {
		return fmt.Errorf("could not bind pflags to viper: %s", err)
	}
	pflag.Parse()

	if err := v.Viper.ReadInConfig(); err != nil {
		return fmt.Errorf("could not read configuration file: %s", err)
	}

	return nil
}

// Get proxies the call to viper's get
func (v *ViperEnv) Get(key string) interface{} {
	return v.Viper.Get(key)
}

// Set proxies the call to viper's set
func (v *ViperEnv) Set(key string, value interface{}) {
	v.Viper.Set(key, value)
}

// Unmarshal introduces the structure provided by value and proxies to viper's unmarshal
func (v *ViperEnv) Unmarshal(value interface{}) error {
	if err := v.introduce(value); err != nil {
		return err
	}
	return v.Viper.Unmarshal(value)
}

// introduce introduces the structure's fields as viper properties.
func (v *ViperEnv) introduce(value interface{}) error {
	var properties []string
	traverseFields(value, "", &properties)
	for _, property := range properties {
		if err := v.Viper.BindEnv(property); err != nil {
			return err
		}
	}
	return nil
}

// traverseFields traverses the provided structure and prepares a slice of strings that contains
// the paths to the structure fields
func traverseFields(value interface{}, buffer string, result *[]string) {
	if !structs.IsStruct(value) {
		index := strings.LastIndex(buffer, ".")
		if index == -1 {
			index = 0
		}
		*result = append(*result, strings.ToLower(buffer[0:index]))
		return
	}
	s := structs.New(value)
	for _, field := range s.Fields() {
		if field.IsExported() {
			if !field.IsEmbedded() {
				buffer += field.Name() + "."
			}
			traverseFields(field.Value(), buffer, result)
			if !field.IsEmbedded() {
				buffer = buffer[0:strings.LastIndex(buffer, field.Name())]
			}
		}
	}
}
