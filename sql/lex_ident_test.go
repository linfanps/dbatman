package sql

import (
	"testing"
)

func TestIdentifier(t *testing.T) {
	testMatchReturn(t, "`test ` ", IDENT_QUOTED, false)
}

func TestMultiIdentifier(t *testing.T) {
	str := "SELECT INSERT 'string     ' UPDATE DELEte `SELECT` `Update`"
	lex, lval := getLexer(str)

	lexExpect(t, lex, lval, SELECT_SYM)
	lexExpect(t, lex, lval, INSERT)

	lexExpect(t, lex, lval, TEXT_STRING)
	lvalExpect(t, lval, "'string     '")

	lexExpect(t, lex, lval, UPDATE_SYM)
	lexExpect(t, lex, lval, DELETE_SYM)

	lexExpect(t, lex, lval, IDENT_QUOTED)
	lvalExpect(t, lval, "`SELECT`")

	lexExpect(t, lex, lval, IDENT_QUOTED)
	lvalExpect(t, lval, "`Update`")

	lexExpect(t, lex, lval, END_OF_INPUT)
}

func TestMultiIdentifier1(t *testing.T) {
	str := "s n insert `s` `` s"
	lex, lval := getLexer(str)

	lexExpect(t, lex, lval, IDENT)
	lvalExpect(t, lval, `s`)

	lexExpect(t, lex, lval, IDENT)
	lvalExpect(t, lval, `n`)

	lexExpect(t, lex, lval, INSERT)

	lexExpect(t, lex, lval, IDENT_QUOTED)
	lvalExpect(t, lval, "`s`")

	lexExpect(t, lex, lval, IDENT_QUOTED)
	lvalExpect(t, lval, "``")

	lexExpect(t, lex, lval, IDENT)
	lvalExpect(t, lval, `s`)
}