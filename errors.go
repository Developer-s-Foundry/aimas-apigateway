package main

import "errors"

var ErrorConfigFileNotFound = errors.New("config file is missing")
var ErrorConfigMissingPort = errors.New("missing port number")
