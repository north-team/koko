package srvconn

import (
	"fmt"
	"io"
)

type ServerConnection interface {
	io.ReadWriteCloser
	SetWinSize(width, height int) error
	KeepAlive() error
}

type Windows struct {
	Width  int
	Height int
}

type sqlOption struct {
	Username string
	Password string
	DBName   string
	Host     string
	Port     int
}

func (opt *sqlOption) CommandArgs() []string {
	return []string{
		fmt.Sprintf("--user=%s", opt.Username),
		fmt.Sprintf("--host=%s", opt.Host),
		fmt.Sprintf("--port=%d", opt.Port),
		"--password",
		opt.DBName,
	}
}

func (opt *sqlOption) Envs() []string {
	var nodename string
	if len(opt.DBName) > 8 {
		nodename = opt.DBName[:8]
	} else {
		nodename = opt.DBName
	}
	return []string{
		fmt.Sprintf("USERNAME=%s", opt.Username),
		fmt.Sprintf("HOSTNAME=%s", opt.Host),
		fmt.Sprintf("PORT=%d", opt.Port),
		fmt.Sprintf("DATABASE=%s", opt.DBName),
		fmt.Sprintf("NODENAME=%s", nodename),
	}
}

type SqlOption func(*sqlOption)

func SqlUsername(username string) SqlOption {
	return func(args *sqlOption) {
		args.Username = username
	}
}

func SqlPassword(password string) SqlOption {
	return func(args *sqlOption) {
		args.Password = password
	}
}

func SqlDBName(dbName string) SqlOption {
	return func(args *sqlOption) {
		args.DBName = dbName
	}
}

func SqlHost(host string) SqlOption {
	return func(args *sqlOption) {
		args.Host = host
	}
}

func SqlPort(port int) SqlOption {
	return func(args *sqlOption) {
		args.Port = port
	}
}
