package proxy

import (
	. "github.com/bytedance/dbatman/database/mysql"
	"github.com/bytedance/dbatman/database/sql"
	"github.com/bytedance/dbatman/hack"
	"github.com/bytedance/dbatman/parser"
)

func (session *Session) handleShow(sqlstmt string, stmt parser.IShow) error {
	var err error

	switch stmt.(type) {
	case *parser.ShowDatabases:
		err = session.handleShowDatabases()
	default:
		err = session.handleQuery(stmt, sqlstmt)
	}

	if err != nil {
		return session.handleMySQLError(err)
	}

	return nil
}

func (session *Session) handleShowDatabases() error {
	dbs := make([]interface{}, 0, 1)
	dbs = append(dbs, session.user.DBName)

	if r, err := session.buildSimpleShowResultset(dbs, "Database"); err != nil {
		return err
	} else {
		return session.WriteRows(r)
	}
}

func (session *Session) buildSimpleShowResultset(values []interface{}, name string) (sql.Rows, error) {

	r := new(SimpleRows)

	r.Cols = []*MySQLField{
		&MySQLField{
			Name:      hack.Slice(name),
			Charset:   uint16(session.fc.Collation()),
			FieldType: FieldTypeVarString,
		},
	}

	var row []byte
	var err error

	for _, value := range values {
		row, err = formatValue(value)
		if err != nil {
			return nil, err
		}

		r.Rows = append(r.Rows, AppendLengthEncodedString(make([]byte, 0, len(row)+9), row))
	}

	return r, nil
}
