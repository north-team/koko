package srvconn

import (
	"bytes"
	"fmt"
	"github.com/jumpserver/koko/pkg/localcommand"
	"github.com/jumpserver/koko/pkg/logger"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"
)

const (
	db2ShellFilename              = "db2"
	db2CatalogNodeShellFilename   = "db2catalognode"
	db2UnCatalogNodeShellFilename = "db2uncatalognode"
	db2CatalogDBShellFilename     = "db2catalogdb"
)

var (
	db2ShellPath              = ""
	db2CatalogNodeShellPath   = ""
	db2UnCatalogNodeShellPath = ""
	db2CatalogDBShellPath     = ""

	_ ServerConnection = (*DB2Conn)(nil)
)

const db2CatalogNodeTemplate = `#!/bin/bash
set -e
exec su - db2inst1 /bin/bash --command="db2 catalog tcpip node ${NODENAME} remote ${HOSTNAME} server ${PORT}"
`

const db2UnCatalogNodeTemplate = `#!/bin/bash
set -e
exec su - db2inst1 /bin/bash --command="db2 uncatalog node ${NODENAME}"
`

const db2CatalogDBTemplate = `#!/bin/bash
set -e
exec su - db2inst1 /bin/bash --command="db2 catalog db ${DATABASE} at node ${NODENAME}"
`

const db2Template = `#!/bin/bash
set - e
exec su - db2inst1 /bin/bash --command="db2"
`

var db2Once sync.Once

func NewDB2Connection(ops ...SqlOption) *DB2Conn {
	args := &sqlOption{
		Username: os.Getenv("USER"),
		Password: os.Getenv("PASSWORD"),
		Host:     "127.0.0.1",
		Port:     50000,
		DBName:   "",
	}
	for _, setter := range ops {
		setter(args)
	}
	return &DB2Conn{options: args}
}

type DB2Conn struct {
	options *sqlOption
	*localcommand.LocalCommand
}

func (conn *DB2Conn) Connect(win Windows) (err error) {
	lcmd, err := startDB2Command(conn)
	if err != nil {
		logger.Errorf("Start db2 command err: %s", err)
		return err
	}
	_ = lcmd.SetWinSize(win.Width, win.Height)
	conn.LocalCommand = lcmd
	logger.Infof("Connect db2 database %s success ", conn.options.Host)
	return
}

func (conn *DB2Conn) KeepAlive() error {
	return nil
}

func (conn *DB2Conn) Close() error {
	_, _ = conn.Write([]byte("quit\r\n"))
	return conn.LocalCommand.Close()
}

func startDB2Command(dbcon *DB2Conn) (lcmd *localcommand.LocalCommand, err error) {
	initOnceLinuxDB2ShellFile()
	if db2ShellPath != "" {
		if lcmd, err = startDB2NameSpaceCommand(dbcon.options); err == nil {
			if lcmd, err = tryManualLoginDB2Server(dbcon, lcmd); err == nil {
				return lcmd, nil
			}
		}
	}
	if lcmd, err = startDB2NormalCommand(dbcon.options); err != nil {
		return nil, err
	}
	return tryManualLoginDB2Server(dbcon, lcmd)

}

func startDB2NameSpaceCommand(opt *sqlOption) (*localcommand.LocalCommand, error) {
	lcmd, _ := localcommand.New(db2CatalogNodeShellPath, opt.CommandArgs(), localcommand.WithEnv(opt.Envs()))
	prompt := [8]byte{}
	nr, _ := lcmd.Read(prompt[:])
	if bytes.Equal(prompt[:nr], []byte("SQL1018N")) {
		lcmd, _ = localcommand.New(db2UnCatalogNodeShellPath, opt.CommandArgs(), localcommand.WithEnv(opt.Envs()))
		_ = lcmd.Close()
		lcmd, _ = localcommand.New(db2CatalogNodeShellPath, opt.CommandArgs(), localcommand.WithEnv(opt.Envs()))
		_ = lcmd.Close()
	}
	lcmd, _ = localcommand.New(db2CatalogDBShellPath, opt.CommandArgs(), localcommand.WithEnv(opt.Envs()))
	res := [1024]byte{}
	_, _ = lcmd.Read(res[:])
	_ = lcmd.Close()

	argv := []string{
		"--fork",
		"--pid",
		"--mount-proc",
		db2ShellPath,
	}
	return localcommand.New(db2ShellPath, argv, localcommand.WithEnv(opt.Envs()))
}

func startDB2NormalCommand(opt *sqlOption) (*localcommand.LocalCommand, error) {
	// 使用 nobody 用户的权限
	nobody, err := user.Lookup("nobody")
	if err != nil {
		logger.Errorf("lookup nobody user err: %s", err)
		return nil, err
	}
	uid, _ := strconv.Atoi(nobody.Uid)
	gid, _ := strconv.Atoi(nobody.Gid)

	return localcommand.New("db2", opt.CommandArgs(), localcommand.WithEnv(opt.Envs()),
		localcommand.WithCmdCredential(&syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}))
}

func tryManualLoginDB2Server(conn *DB2Conn, lcmd *localcommand.LocalCommand) (*localcommand.LocalCommand, error) {
	var (
		err    error
		cmd    string
		buffer bytes.Buffer
	)
	cmd = fmt.Sprintf("connect to %s user %s using %s\n", conn.options.DBName, conn.options.Username, conn.options.Password)
	buffer.WriteString(cmd)
	_, err = lcmd.Write(buffer.Bytes())

	if err != nil {
		_ = lcmd.Close()
		logger.Errorf("Mysql local pty fd read err: %s", err)
		return lcmd, err
	}

	temp := make([]byte, len(cmd))
	_, err = lcmd.Read(temp[:])

	return lcmd, nil
}

func initOnceLinuxDB2ShellFile() {
	db2Once.Do(func() {
		// Linux系统 初始化 DB2 命令文件
		switch runtime.GOOS {
		case "linux":
			if dir, err := os.Getwd(); err == nil {
				TmpDB2ShellPath := filepath.Join(dir, db2ShellFilename)
				if _, err := os.Stat(TmpDB2ShellPath); err == nil {
					db2ShellPath = TmpDB2ShellPath
					logger.Infof("Already init DB2 bash file: %s", TmpDB2ShellPath)
				} else {
					err = ioutil.WriteFile(TmpDB2ShellPath, []byte(db2Template), os.FileMode(0755))
					if err != nil {
						logger.Errorf("Init DB2 bash file failed: %s", err)
						return
					}
					db2ShellPath = TmpDB2ShellPath
					logger.Infof("Init DB2 bash file: %s", db2ShellPath)
				}
				// determine whether the node catalog file exist
				TmpDB2CatalogNodeShellPath := filepath.Join(dir, db2CatalogNodeShellFilename)
				if _, err := os.Stat(TmpDB2CatalogNodeShellPath); err == nil {
					db2CatalogNodeShellPath = TmpDB2CatalogNodeShellPath
					logger.Infof("Already init DB2 catalog1 bash file: %s", db2CatalogNodeShellPath)
				} else {
					err = ioutil.WriteFile(TmpDB2CatalogNodeShellPath, []byte(db2CatalogNodeTemplate), os.FileMode(0755))
					if err != nil {
						logger.Errorf("Init DB2 catalog1 bash file failed: %s", err)
						return
					}
					db2CatalogNodeShellPath = TmpDB2CatalogNodeShellPath
					logger.Infof("Init Catalog1 bash file: %s", db2CatalogNodeShellPath)
				}
				// determine whether the db catalog file exist
				TmpDB2CatalogDBShellPath := filepath.Join(dir, db2CatalogDBShellFilename)
				if _, err := os.Stat(TmpDB2CatalogDBShellPath); err == nil {
					db2CatalogDBShellPath = TmpDB2CatalogDBShellPath
					logger.Infof("Already init DB2 catalog2 bash file: %s", db2CatalogDBShellPath)
				} else {
					err = ioutil.WriteFile(TmpDB2CatalogDBShellPath, []byte(db2CatalogDBTemplate), os.FileMode(0755))
					if err != nil {
						logger.Errorf("Init DB2 catalog2 bash file failed: %s", err)
						return
					}
					db2CatalogDBShellPath = TmpDB2CatalogDBShellPath
					logger.Infof("Init Catalog2 bash file: %s", db2CatalogDBShellPath)
				}
				// determine whether the node uncatalog file exist
				TmpDB2UnCatalogNodeShellPath := filepath.Join(dir, db2UnCatalogNodeShellFilename)
				if _, err := os.Stat(TmpDB2UnCatalogNodeShellPath); err == nil {
					db2UnCatalogNodeShellPath = TmpDB2UnCatalogNodeShellPath
					logger.Infof("Already init DB2 uncatalog1 bash file: %s", db2UnCatalogNodeShellPath)
				} else {
					err = ioutil.WriteFile(TmpDB2UnCatalogNodeShellPath, []byte(db2UnCatalogNodeTemplate), os.FileMode(0755))
					if err != nil {
						logger.Errorf("Init DB2 uncatalog1 bash file failed: %s", err)
						return
					}
					db2UnCatalogNodeShellPath = TmpDB2UnCatalogNodeShellPath
					logger.Infof("Init Catalog1 bash file: %s", db2UnCatalogNodeShellPath)
				}
			}
		}
	})
}
