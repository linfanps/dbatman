package sql

import (
	"testing"
)

func TestTableMtStmt(t *testing.T) {
	st := testParse(`analyze table db1.tb1`, t, false)
	matchType(t, st, &Analyze{})
	matchSchemas(t, st, `db1`)

	st = testParse(`CHECK TABLE test.test_table FAST QUICK;`, t, false)
	matchType(t, st, &Check{})
	matchSchemas(t, st, `test`)

	st = testParse(`CHECKSUM TABLE test.test_table QUICK;`, t, false)
	matchType(t, st, &CheckSum{})
	matchSchemas(t, st, `test`)

	st = testParse(`OPTIMIZE TABLE foo.bar`, t, false)
	matchType(t, st, &Optimize{})
	matchSchemas(t, st, `foo`)

	st = testParse(`REPAIR NO_WRITE_TO_BINLOG  TABLE foo.bar quick`, t, false)
	matchType(t, st, &Repair{})
	matchSchemas(t, st, `foo`)

}

func TestPluginAndUdf(t *testing.T) {
	st := testParse(`CREATE AGGREGATE FUNCTION function_name RETURNS DECIMAL SONAME 'shared_library_name'`, t, false)
	matchType(t, st, &CreateUDF{})

	st = testParse(`INSTALL PLUGIN plugin_name SONAME 'shared_library_name'`, t, false)
	matchType(t, st, &Install{})
	if _, ok := st.(IPluginAndUdf); !ok {
		t.Fatalf("type[%T] is not a instance of IPluginAndUdf", st)
	}

	st = testParse(`UNINSTALL PLUGIN plugin_name`, t, false)
	matchType(t, st, &Uninstall{})
	if _, ok := st.(IPluginAndUdf); !ok {
		t.Fatalf("type[%T] is not a instance of IPluginAndUdf", st)
	}
}

func TestAccountMgrStmt(t *testing.T) {
	st := testParse(`ALTER USER 'jeffrey'@'localhost' PASSWORD EXPIRE;`, t, false)
	matchType(t, st, &AlterUser{})

	st = testParse(`CREATE USER 'jeffrey'@'localhost' IDENTIFIED BY 'mypass';`, t, false)
	matchType(t, st, &CreateUser{})

	st = testParse(`DROP USER 'jeffrey'@'localhost';`, t, false)
	matchType(t, st, &DropUser{})

	st = testParse(`GRANT SELECT ON db2.invoice TO 'jeffrey'@'localhost';`, t, false)
	matchType(t, st, &Grant{})

	st = testParse(`RENAME USER 'jeffrey'@'localhost' TO 'jeff'@'127.0.0.1';`, t, false)
	matchType(t, st, &RenameUser{})

	st = testParse(`REVOKE INSERT ON *.* FROM 'jeffrey'@'localhost';`, t, false)
	matchType(t, st, &Revoke{})

	st = testParse(`SET PASSWORD FOR 'jeffrey'@'localhost' = PASSWORD('cleartext password');`, t, false)
	// matchType(t, st, &SetPassword{})
}
