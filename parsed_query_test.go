/*
Copyright 2017 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sqlparser

import (
	"reflect"
	"testing"

	"github.com/yext/sqlparser/dependency/sqltypes"

	"github.com/yext/sqlparser/dependency/querypb"
)

func TestNewParsedQuery(t *testing.T) {
	stmt, err := Parse("select * from a where id = ?")
	if err != nil {
		t.Error(err)
		return
	}
	pq := NewParsedQuery(stmt)
	want := &ParsedQuery{
		Query:         "select * from a where id = :v1",
		bindLocations: []bindLocation{{offset: 27, length: 3}},
	}
	if !reflect.DeepEqual(pq, want) {
		t.Errorf("GenerateParsedQuery: %+v, want %+v", pq, want)
	}
}

func TestGenerateQuery(t *testing.T) {
	tcases := []struct {
		desc     string
		query    string
		bindVars map[string]*querypb.BindVariable
		extras   map[string]Encodable
		output   string
	}{
		{
			desc:  "no substitutions",
			query: "select * from a where id = 2",
			bindVars: map[string]*querypb.BindVariable{
				"v1": sqltypes.Int64BindVariable(1),
			},
			output: "select * from a where id = 2",
		}, {
			desc:  "missing bind var",
			query: "select * from a where id1 = ? and id2 = ?",
			bindVars: map[string]*querypb.BindVariable{
				"v1": sqltypes.Int64BindVariable(1),
			},
			output: "missing bind var v2",
		}, {
			desc:  "simple bindvar substitution",
			query: "select * from a where id1 = ? and id2 = ?",
			bindVars: map[string]*querypb.BindVariable{
				"v1": sqltypes.Int64BindVariable(1),
				"v2": sqltypes.NullBindVariable,
			},
			output: "select * from a where id1 = 1 and id2 = null",
		}, {
			desc:  "non-list bind var supplied",
			query: "select * from a where id = ?",
			bindVars: map[string]*querypb.BindVariable{
				"v1": sqltypes.TestBindVariable([]interface{}{1}),
			},
			output: "unexpected arg type (TUPLE) for non-list key v1",
		}, {
			desc:  "single column tuple equality",
			query: "select * from a where b = ?",
			extras: map[string]Encodable{
				"v1": &TupleEqualityList{
					Columns: []ColIdent{NewColIdent("pk")},
					Rows: [][]sqltypes.Value{
						{sqltypes.NewInt64(1)},
						{sqltypes.NewVarBinary("aa")},
					},
				},
			},
			output: "select * from a where b = pk in (1, 'aa')",
		}, {
			desc:  "multi column tuple equality",
			query: "select * from a where b = ?",
			extras: map[string]Encodable{
				"v1": &TupleEqualityList{
					Columns: []ColIdent{NewColIdent("pk1"), NewColIdent("pk2")},
					Rows: [][]sqltypes.Value{
						{
							sqltypes.NewInt64(1),
							sqltypes.NewVarBinary("aa"),
						},
						{
							sqltypes.NewInt64(2),
							sqltypes.NewVarBinary("bb"),
						},
					},
				},
			},
			output: "select * from a where b = (pk1 = 1 and pk2 = 'aa') or (pk1 = 2 and pk2 = 'bb')",
		},
	}

	for _, tcase := range tcases {
		tree, err := Parse(tcase.query)
		if err != nil {
			t.Errorf("parse failed for %s: %v", tcase.desc, err)
			continue
		}
		buf := NewTrackedBuffer(nil)
		buf.Myprintf("%v", tree)
		pq := buf.ParsedQuery()
		bytes, err := pq.GenerateQuery(tcase.bindVars, tcase.extras)
		var got string
		if err != nil {
			got = err.Error()
		} else {
			got = string(bytes)
		}
		if got != tcase.output {
			t.Errorf("for test case: %s, got: '%s', want '%s'", tcase.desc, got, tcase.output)
		}
	}
}
