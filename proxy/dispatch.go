// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"fmt"
	"github.com/bytedance/dbatman/database/cluster"
	. "github.com/bytedance/dbatman/database/mysql"
	"github.com/bytedance/dbatman/database/sql"
	"github.com/bytedance/dbatman/database/sql/driver"
	"github.com/bytedance/dbatman/errors"
	"github.com/bytedance/dbatman/hack"
	"github.com/ngaut/log"
	"io"
)

func (session *Session) dispatch(data []byte) error {
	cmd := data[0]
	data = data[1:]

	// https://dev.mysql.com/doc/internals/en/sequence-id.html
	//defer func() {
	//	if session.lastcmd != cmd {
	//		session.lastcmd = cmd
	//		session.fc.ResetSequence()
	//	}
	//}()

	switch cmd {
	case ComQuit:
		session.Close()
		return nil
	case ComQuery:
		return session.comQuery(hack.String(data))
	case ComPing:
		return session.fc.WriteOK(nil)
	case ComInitDB:
		if err := session.useDB(hack.String(data)); err != nil {
			return session.handleMySQLError(err)
		} else {
			return session.fc.WriteOK(nil)
		}
	case ComFieldList:
		// return session.handleFieldList(data)
		// TODO
		return nil
	case ComStmtPrepare:
		return session.handleComStmtPrepare(hack.String(data))
	case ComStmtExecute:
		return session.handleComStmtExecute(data)
	case ComStmtClose:
		return session.handleComStmtClose(data)
	case ComStmtSendLongData:
		// TODO
		//return session.handleComStmtSendLongData(data)
	case ComStmtReset:
		// TODO
		// return session.handleComStmtReset(data)
	default:
		msg := fmt.Sprintf("command %d not supported now", cmd)
		log.Warnf(msg)
		return NewDefaultError(ER_UNKNOWN_ERROR, msg)
	}

	return nil
}

func (session *Session) useDB(db string) error {

	if session.cluster != nil {
		if session.cluster.DBName != db {
			return NewDefaultError(ER_BAD_DB_ERROR, db)
		}

		return nil
	}

	if _, err := session.config.GetClusterByDBName(db); err != nil {
		return NewDefaultError(ER_BAD_DB_ERROR, db)
	} else if session.cluster, err = cluster.New(session.user.ClusterName); err != nil {
		return err
	}

	if session.bc == nil {
		master, err := session.cluster.Master()
		if err != nil {
			return NewDefaultError(ER_BAD_DB_ERROR, db)
		}
		slave, err := session.cluster.Slave()
		if err != nil {
			slave = master
		}
		session.bc = &SqlConn{
			master:  master,
			slave:   slave,
			stmts:   make(map[uint32]*sql.Stmt),
			tx:      nil,
			session: session,
		}
	}

	return nil
}

func (session *Session) IsAutoCommit() bool {
	return session.fc.Status()&uint16(StatusInAutocommit) > 0
}

func (session *Session) WriteRows(rs sql.Rows) error {
	var cols []driver.RawPacket
	var err error
	cols, err = rs.ColumnPackets()

	if err != nil {
		return session.handleMySQLError(err)
	}

	// Send a packet contains column length
	data := make([]byte, 4, 32)
	data = AppendLengthEncodedInteger(data, uint64(len(cols)))
	if err = session.fc.WritePacket(data); err != nil {
		return errors.Trace(err)
	}

	// Write Columns Packet
	for _, col := range cols {
		if err := session.fc.WritePacket(col); err != nil {
			log.Debugf("write columns packet error %v", err)
			return errors.Trace(err)
		}
	}

	// TODO Write a ok packet
	if err = session.fc.WriteEOF(); err != nil {
		return errors.Trace(err)
	}

	for {
		packet, err := rs.NextRowPacket()

		// Handle Error
		rerr := errors.Real(err)

		if rerr != nil {
			if rerr == io.EOF {
				return session.fc.WriteEOF()
			} else {
				return session.handleMySQLError(rerr)
			}
		}

		if err := session.fc.WritePacket(packet); err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}

func (session *Session) handleMySQLError(e error) error {

	err := errors.Real(e)

	switch inst := err.(type) {
	case *MySQLError:
		session.fc.WriteError(inst)
		return nil
	case *MySQLWarnings:
		// TODO process warnings
		log.Debugf("warnings %v", inst)
		session.fc.WriteOK(nil)
		return nil
	default:
		log.Warnf("default error: %T %s", err, errors.ErrorStack(e))
		return err
	}
}
