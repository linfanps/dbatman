package proxy

import (
	"encoding/binary"
	"fmt"
	. "github.com/bytedance/dbatman/database/mysql"
	"github.com/bytedance/dbatman/database/sql"
	"github.com/bytedance/dbatman/database/sql/driver"
	"github.com/bytedance/dbatman/errors"
	"github.com/bytedance/dbatman/parser"
	"github.com/ngaut/log"
	"strconv"
)

func (c *Session) handleComStmtPrepare(sqlstmt string) error {
	stmt, err := parser.Parse(sqlstmt)
	if err != nil {
		log.Warningf(`parse sql "%s" error "%s"`, sqlstmt, err.Error())
		return c.handleMySQLError(
			NewDefaultError(ER_SYNTAX_ERROR, err.Error()))
	}

	// Only a few statements supported by prepare statements
	// http://dev.mysql.com/worklog/task/?id=2871
	switch v := stmt.(type) {
	case parser.ISelect, *parser.Insert, *parser.Update, *parser.Delete, *parser.Replace:
		return c.prepare(v, sqlstmt)
	case parser.IDDLStatement:
		// return c.prepareDDL(v, sqlstmt)
		return nil
	default:
		log.Warnf("statement %T[%s] not support prepare ops", stmt, sqlstmt)
		return c.handleMySQLError(
			NewDefaultError(ER_UNSUPPORTED_PS))
	}
}

func (session *Session) prepare(istmt parser.IStatement, sqlstmt string) error {
	if err := session.checkDB(istmt); err != nil {
		log.Debugf("check db error: %s", err.Error())
		return err
	}

	isread := false

	if s, ok := istmt.(parser.ISelect); ok {
		isread = !s.IsLocked()
	}

	if session.isInTransaction() || !session.isAutoCommit() {
		isread = false
	}

	stmt, err := session.Executor(isread).Prepare(sqlstmt)
	// TODO here should handler error
	if err != nil {
		return session.handleMySQLError(err)
	}

	//	record the sql
	stmt.SQL = istmt

	// TODO duplicate
	session.bc.stmts[stmt.ID] = stmt

	return session.writePrepareResult(stmt)
}

func (session *Session) writePrepareResult(stmt *sql.Stmt) error {

	colen := len(stmt.Columns)
	paramlen := len(stmt.Params)

	// Prepare Header
	header := make([]byte, PacketHeaderLen, 12+PacketHeaderLen)

	// OK Status
	header = append(header, 0)
	header = append(header, byte(stmt.ID), byte(stmt.ID>>8), byte(stmt.ID>>16), byte(stmt.ID>>24))

	header = append(header, byte(colen), byte(colen>>8))
	header = append(header, byte(paramlen), byte(paramlen>>8))

	// reserved 00
	header = append(header, 0)

	// warning count 00
	// TODO
	header = append(header, 0, 0)

	if err := session.fc.WritePacket(header); err != nil {
		return session.handleMySQLError(err)
	}

	if paramlen > 0 {
		for _, p := range stmt.Params {
			if err := session.fc.WritePacket(p); err != nil {
				return session.handleMySQLError(err)
			}
		}

		if err := session.fc.WriteEOF(); err != nil {
			return session.handleMySQLError(err)
		}

	}

	if colen > 0 {
		for _, c := range stmt.Columns {
			if err := session.fc.WritePacket(c); err != nil {
				return session.handleMySQLError(err)
			}
		}

		if err := session.fc.WriteEOF(); err != nil {
			return session.handleMySQLError(err)
		}
	}

	return nil
}

func (session *Session) handleComStmtExecute(data []byte) error {

	if len(data) < 9 {
		return session.handleMySQLError(ErrMalformPkt)
	}

	pos := 0
	id := binary.LittleEndian.Uint32(data[0:4])
	pos += 4

	stmt, ok := session.bc.stmts[id]
	if !ok {
		return NewDefaultError(ER_UNKNOWN_STMT_HANDLER,
			strconv.FormatUint(uint64(id), 10), "stmt_execute")
	}

	flag := data[pos]
	pos++

	//now we only support CURSOR_TYPE_NO_CURSOR flag
	if flag != 0 {
		return NewDefaultError(ER_UNKNOWN_ERROR, fmt.Sprintf("unsupported flag %d", flag))
	}

	//skip iteration-count, always 1
	pos += 4

	var err error
	if _, ok := stmt.SQL.(parser.ISelect); ok {
		err = session.handleStmtQuery(stmt, data[pos:])
	} else {
		err = session.handleStmtExec(stmt, data[pos:])
	}

	return errors.Trace(err)

}

func (session *Session) handleStmtExec(stmt *sql.Stmt, data []byte) error {

	var rs sql.Result
	var err error

	if len(data) > 0 {
		rs, err = stmt.Exec(driver.RawStmtParams(data))
	} else {
		rs, err = stmt.Exec()
	}

	if err != nil {
		return session.handleMySQLError(err)
	}

	return session.fc.WriteOK(rs)
}

func (session *Session) handleStmtQuery(stmt *sql.Stmt, data []byte) error {
	var rows sql.Rows
	var err error

	if len(data) > 0 {
		rows, err = stmt.Query(driver.RawStmtParams(data))
	} else {
		rows, err = stmt.Query()
	}

	if err != nil {
		return session.handleMySQLError(err)
	}

	return session.WriteRows(rows)
}

/*
func (c *Session) handleComStmtSendLongData(data []byte) error {
	if len(data) < 6 {
		AppLog.Warn("ErrMalFormPacket")
		return ErrMalformPacket
	}

	id := binary.LittleEndian.Uint32(data[0:4])

	s, ok := c.stmts[id]
	if !ok {
		return NewDefaultError(ER_UNKNOWN_STMT_HANDLER,
			strconv.FormatUint(uint64(id), 10), "stmt_send_longdata")
	}

	paramId := binary.LittleEndian.Uint16(data[4:6])
	if paramId >= uint16(s.params) {
		return NewDefaultError(ER_WRONG_ARGUMENTS, "stmt_send_longdata")
	}

	s.cstmt.SendLongData(paramId, data[6:])
	return nil
}

func (c *Session) handleComStmtReset(data []byte) error {
	if len(data) < 4 {
		AppLog.Warn("ErrMalFormPacket")
		return ErrMalformPacket
	}

	id := binary.LittleEndian.Uint32(data[0:4])

	s, ok := c.stmts[id]
	if !ok {
		return NewDefaultError(ER_UNKNOWN_STMT_HANDLER,
			strconv.FormatUint(uint64(id), 10), "stmt_reset")
	}

	if r, err := s.cstmt.Reset(); err != nil {
		return err
	} else {
		s.ClearParams()
		return c.writeOK(r)
	}
}

*/

func (c *Session) handleComStmtClose(data []byte) error {
	if len(data) < 4 {
		return nil
	}

	id := binary.LittleEndian.Uint32(data[0:4])

	if cstmt, ok := c.bc.stmts[id]; ok {
		cstmt.Close()
	}

	delete(c.bc.stmts, id)

	return nil
}
