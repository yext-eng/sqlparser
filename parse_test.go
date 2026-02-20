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
	"bytes"
	"fmt"
	"strings"
	"testing"
)

var (
	validSQL = []struct {
		input  string
		output string
	}{{
		input:  "select 1",
		output: "select 1 from dual",
	}, {
		input: "select 1 from t",
	}, {
		input: "select .1 from t",
	}, {
		input: "select 1.2e1 from t",
	}, {
		input: "select 1.2e+1 from t",
	}, {
		input: "select 1.2e-1 from t",
	}, {
		input: "select 08.3 from t",
	}, {
		input: "select -1 from t where b = -2",
	}, {
		input:  "select - -1 from t",
		output: "select 1 from t",
	}, {
		input:  "select 1 from t // aa\n",
		output: "select 1 from t",
	}, {
		input:  "select 1 from t -- aa\n",
		output: "select 1 from t",
	}, {
		input:  "select 1 from t # aa\n",
		output: "select 1 from t",
	}, {
		input:  "select 1 --aa\nfrom t",
		output: "select 1 from t",
	}, {
		input:  "select 1 #aa\nfrom t",
		output: "select 1 from t",
	}, {
		input: "select /* simplest */ 1 from t",
	}, {
		input: "select /* double star **/ 1 from t",
	}, {
		input: "select /* double */ /* comment */ 1 from t",
	}, {
		input: "select /* back-quote keyword */ `By` from t",
	}, {
		input: "select /* back-quote num */ `2a` from t",
	}, {
		input: "select /* back-quote . */ `a.b` from t",
	}, {
		input: "select /* back-quote back-quote */ `a``b` from t",
	}, {
		input:  "select /* back-quote unnecessary */ 1 from `t`",
		output: "select /* back-quote unnecessary */ 1 from t",
	}, {
		input:  "select /* back-quote idnum */ 1 from `a1`",
		output: "select /* back-quote idnum */ 1 from a1",
	}, {
		input: "select /* @ */ @@a from b",
	}, {
		input: "select /* \\0 */ '\\0' from a",
	}, {
		input:  "select 1 /* drop this comment */ from t",
		output: "select 1 from t",
	}, {
		input: "select /* union */ 1 from t union select 1 from t",
	}, {
		input: "select /* double union */ 1 from t union select 1 from t union select 1 from t",
	}, {
		input: "select /* union all */ 1 from t union all select 1 from t",
	}, {
		input: "select /* union distinct */ 1 from t union distinct select 1 from t",
	}, {
		input: "select /* intersect */ 1 from t intersect select 1 from s",
	}, {
		input: "select /* intersect all */ 1 from t intersect all select 1 from s",
	}, {
		input: "select /* except */ 1 from t except select 1 from s",
	}, {
		input: "select /* except distinct */ 1 from t except distinct select 1 from s",
	}, {
		input: "select /* except all */ 1 from t except all select 1 from s",
	}, {
		input: "select /* union intersect precedence */ 1 from t union select 2 from s intersect select 3 from u",
	}, {
		input: "select /* except intersect precedence */ 1 from t except select 2 from s intersect select 3 from u",
	}, {
		input: "select /* union parenthesized set expr */ 1 from t union (select 2 from s except select 3 from u)",
	}, {
		input:  "select /* except order by limit */ a from t except select a from s order by a limit 2",
		output: "select /* except order by limit */ a from t except select a from s order by a asc limit 2",
	}, {
		input:  "select /* intersect parenthesized rhs */ a from t intersect (select a from s order by a limit 1)",
		output: "select /* intersect parenthesized rhs */ a from t intersect (select a from s order by a asc limit 1)",
	}, {
		input:  "(select /* union parenthesized select */ 1 from t order by a) union select 1 from t",
		output: "(select /* union parenthesized select */ 1 from t order by a asc) union select 1 from t",
	}, {
		input: "select /* union parenthesized select 2 */ 1 from t union (select 1 from t)",
	}, {
		input:  "select /* union order by */ 1 from t union select 1 from t order by a",
		output: "select /* union order by */ 1 from t union select 1 from t order by a asc",
	}, {
		input:  "select /* union order by limit lock */ 1 from t union select 1 from t order by a limit 1 for update",
		output: "select /* union order by limit lock */ 1 from t union select 1 from t order by a asc limit 1 for update",
	}, {
		input: "select /* union with limit on lhs */ 1 from t limit 1 union select 1 from t",
	}, {
		input:  "(select id, a from t order by id limit 1) union (select id, b as a from s order by id limit 1) order by a limit 1",
		output: "(select id, a from t order by id asc limit 1) union (select id, b as a from s order by id asc limit 1) order by a asc limit 1",
	}, {
		input: "select a from (select 1 as a from tbl1 union select 2 from tbl2) as t",
	}, {
		input: "select * from t1 join (select * from t2 union select * from t3) as t",
	}, {
		// Ensure this doesn't generate: ""select * from t1 join t2 on a = b join t3 on a = b".
		input: "select * from t1 join t2 on a = b join t3",
	}, {
		input: "select * from t1 where col in (select 1 from dual union select 2 from dual)",
	}, {
		input: "select * from t1 where exists (select a from t2 union select b from t3)",
	}, {
		input:  "with cte as (select 1) select * from cte",
		output: "with cte as (select 1 from dual) select * from cte",
	}, {
		input:  "with recursive cte(n) as (select 1 union all select n + 1 from cte where n < 5) select n from cte",
		output: "with recursive cte(n) as (select 1 from dual union all select n + 1 from cte where n < 5) select n from cte",
	}, {
		input:  "with a as (select 1), b as (select * from a) select * from b",
		output: "with a as (select 1 from dual), b as (select * from a) select * from b",
	}, {
		input: "select /* distinct */ distinct 1 from t",
	}, {
		input: "select /* straight_join */ straight_join 1 from t",
	}, {
		input: "select /* for update */ 1 from t for update",
	}, {
		input: "select /* for update nowait */ 1 from t for update nowait",
	}, {
		input: "select /* for update skip locked */ 1 from t for update skip locked",
	}, {
		input: "select /* for share */ 1 from t for share",
	}, {
		input: "select /* for share nowait */ 1 from t for share nowait",
	}, {
		input: "select /* for share skip locked */ 1 from t for share skip locked",
	}, {
		input: "select /* lock in share mode */ 1 from t lock in share mode",
	}, {
		input: "select /* select list */ 1, 2 from t",
	}, {
		input: "select /* * */ * from t",
	}, {
		input: "select /* a.* */ a.* from t",
	}, {
		input: "select /* a.b.* */ a.b.* from t",
	}, {
		input:  "select /* column alias */ a b from t",
		output: "select /* column alias */ a as b from t",
	}, {
		input: "select /* column alias with as */ a as b from t",
	}, {
		input: "select /* keyword column alias */ a as `By` from t",
	}, {
		input:  "select /* column alias as string */ a as \"b\" from t",
		output: "select /* column alias as string */ a as b from t",
	}, {
		input:  "select /* column alias as string without as */ a \"b\" from t",
		output: "select /* column alias as string without as */ a as b from t",
	}, {
		input: "select /* a.* */ a.* from t",
	}, {
		input: "select /* `By`.* */ `By`.* from t",
	}, {
		input: "select /* select with bool expr */ a = b from t",
	}, {
		input: "select /* case_when */ case when a = b then c end from t",
	}, {
		input: "select /* case_when_else */ case when a = b then c else d end from t",
	}, {
		input: "select /* case_when_when_else */ case when a = b then c when b = d then d else d end from t",
	}, {
		input: "select /* case */ case aa when a = b then c end from t",
	}, {
		input: "select /* parenthesis */ 1 from (t)",
	}, {
		input: "select /* parenthesis multi-table */ 1 from (t1, t2)",
	}, {
		input: "select /* table list */ 1 from t1, t2",
	}, {
		input: "select /* parenthessis in table list 1 */ 1 from (t1), t2",
	}, {
		input: "select /* parenthessis in table list 2 */ 1 from t1, (t2)",
	}, {
		input: "select /* use */ 1 from t1 use index (a) where b = 1",
	}, {
		input: "select /* keyword index */ 1 from t1 use index (`By`) where b = 1",
	}, {
		input: "select /* ignore */ 1 from t1 as t2 ignore index (a), t3 use index (b) where b = 1",
	}, {
		input: "select /* use */ 1 from t1 as t2 use index (a), t3 use index (b) where b = 1",
	}, {
		input: "select /* force */ 1 from t1 as t2 force index (a), t3 force index (b) where b = 1",
	}, {
		input:  "select /* table alias */ 1 from t t1",
		output: "select /* table alias */ 1 from t as t1",
	}, {
		input: "select /* table alias with as */ 1 from t as t1",
	}, {
		input:  "select /* string table alias */ 1 from t as 't1'",
		output: "select /* string table alias */ 1 from t as t1",
	}, {
		input:  "select /* string table alias without as */ 1 from t 't1'",
		output: "select /* string table alias without as */ 1 from t as t1",
	}, {
		input: "select /* keyword table alias */ 1 from t as `By`",
	}, {
		input: "select /* join */ 1 from t1 join t2",
	}, {
		input: "select /* join on */ 1 from t1 join t2 on a = b",
	}, {
		input: "select /* join on */ 1 from t1 join t2 using (a)",
	}, {
		input:  "select /* inner join */ 1 from t1 inner join t2",
		output: "select /* inner join */ 1 from t1 join t2",
	}, {
		input:  "select /* cross join */ 1 from t1 cross join t2",
		output: "select /* cross join */ 1 from t1 join t2",
	}, {
		input: "select /* straight_join */ 1 from t1 straight_join t2",
	}, {
		input: "select /* straight_join on */ 1 from t1 straight_join t2 on a = b",
	}, {
		input: "select /* left join */ 1 from t1 left join t2 on a = b",
	}, {
		input: "select /* left join */ 1 from t1 left join t2 using (a)",
	}, {
		input:  "select /* left outer join */ 1 from t1 left outer join t2 on a = b",
		output: "select /* left outer join */ 1 from t1 left join t2 on a = b",
	}, {
		input:  "select /* left outer join */ 1 from t1 left outer join t2 using (a)",
		output: "select /* left outer join */ 1 from t1 left join t2 using (a)",
	}, {
		input: "select /* right join */ 1 from t1 right join t2 on a = b",
	}, {
		input: "select /* right join */ 1 from t1 right join t2 using (a)",
	}, {
		input:  "select /* right outer join */ 1 from t1 right outer join t2 on a = b",
		output: "select /* right outer join */ 1 from t1 right join t2 on a = b",
	}, {
		input:  "select /* right outer join */ 1 from t1 right outer join t2 using (a)",
		output: "select /* right outer join */ 1 from t1 right join t2 using (a)",
	}, {
		input: "select /* natural join */ 1 from t1 natural join t2",
	}, {
		input: "select /* natural left join */ 1 from t1 natural left join t2",
	}, {
		input:  "select /* natural left outer join */ 1 from t1 natural left join t2",
		output: "select /* natural left outer join */ 1 from t1 natural left join t2",
	}, {
		input: "select /* natural right join */ 1 from t1 natural right join t2",
	}, {
		input:  "select /* natural right outer join */ 1 from t1 natural right join t2",
		output: "select /* natural right outer join */ 1 from t1 natural right join t2",
	}, {
		input: "select /* join on */ 1 from t1 join t2 on a = b",
	}, {
		input: "select /* join using */ 1 from t1 join t2 using (a)",
	}, {
		input: "select /* join using (a, b, c) */ 1 from t1 join t2 using (a, b, c)",
	}, {
		input: "select /* s.t */ 1 from s.t",
	}, {
		input: "select /* keyword schema & table name */ 1 from `By`.`bY`",
	}, {
		input: "select /* select in from */ 1 from (select 1 from t) as a",
	}, {
		input: "select /* json_table basic */ jt.id from json_table(doc, '$' columns (id int path '$.id')) as jt",
	}, {
		input:  "select /* json_table alias */ * from json_table(payload, '$[*]' columns (rn for ordinality, has_price int exists path '$.price')) jt",
		output: "select /* json_table alias */ * from json_table(payload, '$[*]' columns (rn for ordinality, has_price int exists path '$.price')) as jt",
	}, {
		input: "select /* json_table nested */ * from json_table(doc, '$[*]' columns (id int path '$.id', nested path '$.tags[*]' columns (tag varchar(20) path '$'))) as jt",
	}, {
		input:  "select /* select in from with no as */ 1 from (select 1 from t) a",
		output: "select /* select in from with no as */ 1 from (select 1 from t) as a",
	}, {
		input: "select /* where */ 1 from t where a = b",
	}, {
		input: "select /* and */ 1 from t where a = b and a = c",
	}, {
		input:  "select /* && */ 1 from t where a = b && a = c",
		output: "select /* && */ 1 from t where a = b and a = c",
	}, {
		input: "select /* or */ 1 from t where a = b or a = c",
	}, {
		input:  "select /* || */ 1 from t where a = b || a = c",
		output: "select /* || */ 1 from t where a = b or a = c",
	}, {
		input: "select /* not */ 1 from t where not a = b",
	}, {
		input: "select /* ! */ 1 from t where a = !1",
	}, {
		input: "select /* bool is */ 1 from t where a = b is null",
	}, {
		input: "select /* bool is not */ 1 from t where a = b is not false",
	}, {
		input: "select /* true */ 1 from t where true",
	}, {
		input: "select /* false */ 1 from t where false",
	}, {
		input: "select /* false on left */ 1 from t where false = 0",
	}, {
		input: "select /* exists */ 1 from t where exists (select 1 from t)",
	}, {
		input: "select /* (boolean) */ 1 from t where not (a = b)",
	}, {
		input: "select /* in value list */ 1 from t where a in (b, c)",
	}, {
		input: "select /* in select */ 1 from t where a in (select 1 from t)",
	}, {
		input: "select /* not in */ 1 from t where a not in (b, c)",
	}, {
		input: "select /* like */ 1 from t where a like b",
	}, {
		input: "select /* like escape */ 1 from t where a like b escape '!'",
	}, {
		input: "select /* not like */ 1 from t where a not like b",
	}, {
		input: "select /* not like escape */ 1 from t where a not like b escape '$'",
	}, {
		input: "select /* regexp */ 1 from t where a regexp b",
	}, {
		input: "select /* not regexp */ 1 from t where a not regexp b",
	}, {
		input:  "select /* rlike */ 1 from t where a rlike b",
		output: "select /* rlike */ 1 from t where a regexp b",
	}, {
		input:  "select /* not rlike */ 1 from t where a not rlike b",
		output: "select /* not rlike */ 1 from t where a not regexp b",
	}, {
		input: "select /* member of */ 1 from t where a member of (b)",
	}, {
		input:  "select /* member of uppercase */ 1 from t where a MEMBER OF (b)",
		output: "select /* member of uppercase */ 1 from t where a member of (b)",
	}, {
		input: "select /* not member of */ 1 from t where a not member of (b)",
	}, {
		input: "select /* regexp_like */ regexp_like(a, 'b+') from t",
	}, {
		input: "select /* regexp_instr */ regexp_instr(a, 'b+') from t",
	}, {
		input: "select /* regexp_substr */ regexp_substr(a, 'b+') from t",
	}, {
		input: "select /* regexp_replace */ regexp_replace(a, 'b+', 'c') from t",
	}, {
		input: "select /* between */ 1 from t where a between b and c",
	}, {
		input: "select /* not between */ 1 from t where a not between b and c",
	}, {
		input: "select /* is null */ 1 from t where a is null",
	}, {
		input: "select /* is not null */ 1 from t where a is not null",
	}, {
		input: "select /* is true */ 1 from t where a is true",
	}, {
		input: "select /* is not true */ 1 from t where a is not true",
	}, {
		input: "select /* is false */ 1 from t where a is false",
	}, {
		input: "select /* is not false */ 1 from t where a is not false",
	}, {
		input: "select /* < */ 1 from t where a < b",
	}, {
		input: "select /* <= */ 1 from t where a <= b",
	}, {
		input: "select /* >= */ 1 from t where a >= b",
	}, {
		input: "select /* > */ 1 from t where a > b",
	}, {
		input: "select /* != */ 1 from t where a != b",
	}, {
		input:  "select /* <> */ 1 from t where a <> b",
		output: "select /* <> */ 1 from t where a != b",
	}, {
		input: "select /* <=> */ 1 from t where a <=> b",
	}, {
		input: "select /* != */ 1 from t where a != b",
	}, {
		input: "select /* single value expre list */ 1 from t where a in (b)",
	}, {
		input: "select /* select as a value expression */ 1 from t where a = (select a from t)",
	}, {
		input: "select /* parenthesised value */ 1 from t where a = (b)",
	}, {
		input: "select /* over-parenthesize */ ((1)) from t where ((a)) in (((1))) and ((a, b)) in ((((1, 1))), ((2, 2)))",
	}, {
		input: "select /* dot-parenthesize */ (a.b) from t where (b.c) = 2",
	}, {
		input: "select /* & */ 1 from t where a = b & c",
	}, {
		input: "select /* & */ 1 from t where a = b & c",
	}, {
		input: "select /* | */ 1 from t where a = b | c",
	}, {
		input: "select /* ^ */ 1 from t where a = b ^ c",
	}, {
		input: "select /* + */ 1 from t where a = b + c",
	}, {
		input: "select /* - */ 1 from t where a = b - c",
	}, {
		input: "select /* * */ 1 from t where a = b * c",
	}, {
		input: "select /* / */ 1 from t where a = b / c",
	}, {
		input: "select /* % */ 1 from t where a = b % c",
	}, {
		input: "select /* div */ 1 from t where a = b div c",
	}, {
		input:  "select /* MOD */ 1 from t where a = b MOD c",
		output: "select /* MOD */ 1 from t where a = b % c",
	}, {
		input: "select /* << */ 1 from t where a = b << c",
	}, {
		input: "select /* >> */ 1 from t where a = b >> c",
	}, {
		input:  "select /* % no space */ 1 from t where a = b%c",
		output: "select /* % no space */ 1 from t where a = b % c",
	}, {
		input: "select /* u+ */ 1 from t where a = +b",
	}, {
		input: "select /* u- */ 1 from t where a = -b",
	}, {
		input: "select /* u~ */ 1 from t where a = ~b",
	}, {
		input: "select /* -> */ a.b -> 'ab' from t",
	}, {
		input: "select /* -> */ a.b ->> 'ab' from t",
	}, {
		input: "select /* empty function */ 1 from t where a = b()",
	}, {
		input: "select /* function with 1 param */ 1 from t where a = b(c)",
	}, {
		input: "select /* function with many params */ 1 from t where a = b(c, d)",
	}, {
		input: "select /* function with distinct */ count(distinct a) from t",
	}, {
		input:  "select /* window basic */ row_number() over (partition by dept order by salary) from t",
		output: "select /* window basic */ row_number() over (partition by dept order by salary asc) from t",
	}, {
		input:  "select /* window named */ sum(v) over w from t window w as (partition by dept order by ts rows between unbounded preceding and current row)",
		output: "select /* window named */ sum(v) over w from t window w as (partition by dept order by ts asc rows between unbounded preceding and current row)",
	}, {
		input:  "select /* window inherit */ sum(v) over (w order by ts) from t window w as (partition by dept)",
		output: "select /* window inherit */ sum(v) over (w order by ts asc) from t window w as (partition by dept)",
	}, {
		input:  "select /* window multi */ row_number() over (order by a), rank() over (order by b desc) from t",
		output: "select /* window multi */ row_number() over (order by a asc), rank() over (order by b desc) from t",
	}, {
		input: "select /* if as func */ 1 from t where a = if(b)",
	}, {
		input: "select /* current_timestamp as func */ current_timestamp() from t",
	}, {
		input: "select /* mod as func */ a from tab where mod(b, 2) = 0",
	}, {
		input: "select /* database as func no param */ database() from t",
	}, {
		input: "select /* database as func 1 param */ database(1) from t",
	}, {
		input: "select /* a */ a from t",
	}, {
		input: "select /* a.b */ a.b from t",
	}, {
		input: "select /* a.b.c */ a.b.c from t",
	}, {
		input: "select /* keyword a.b */ `By`.`bY` from t",
	}, {
		input: "select /* string */ 'a' from t",
	}, {
		input:  "select /* double quoted string */ \"a\" from t",
		output: "select /* double quoted string */ 'a' from t",
	}, {
		input:  "select /* quote quote in string */ 'a''a' from t",
		output: "select /* quote quote in string */ 'a\\'a' from t",
	}, {
		input:  "select /* double quote quote in string */ \"a\"\"a\" from t",
		output: "select /* double quote quote in string */ 'a\\\"a' from t",
	}, {
		input:  "select /* quote in double quoted string */ \"a'a\" from t",
		output: "select /* quote in double quoted string */ 'a\\'a' from t",
	}, {
		input: "select /* backslash quote in string */ 'a\\'a' from t",
	}, {
		input: "select /* literal backslash in string */ 'a\\\\na' from t",
	}, {
		input: "select /* all escapes */ '\\0\\'\\\"\\b\\n\\r\\t\\Z\\\\' from t",
	}, {
		input:  "select /* non-escape */ '\\x' from t",
		output: "select /* non-escape */ 'x' from t",
	}, {
		input: "select /* unescaped backslash */ '\\n' from t",
	}, {
		input:  "select /* positional argument */ ? from t",
		output: "select /* positional argument */ :v1 from t",
	}, {
		input:  "select /* multiple positional arguments */ ?, ? from t",
		output: "select /* multiple positional arguments */ :v1, :v2 from t",
	}, {
		input: "select /* null */ null from t",
	}, {
		input: "select /* octal */ 010 from t",
	}, {
		input:  "select /* hex */ x'f0A1' from t",
		output: "select /* hex */ X'f0A1' from t",
	}, {
		input: "select /* hex caps */ X'F0a1' from t",
	}, {
		input:  "select /* bit literal */ b'0101' from t",
		output: "select /* bit literal */ B'0101' from t",
	}, {
		input: "select /* bit literal caps */ B'010011011010' from t",
	}, {
		input: "select /* 0x */ 0xf0 from t",
	}, {
		input: "select /* float */ 0.1 from t",
	}, {
		input: "select /* group by */ 1 from t group by a",
	}, {
		input: "select /* having */ 1 from t having a = b",
	}, {
		input:  "select /* simple order by */ 1 from t order by a",
		output: "select /* simple order by */ 1 from t order by a asc",
	}, {
		input: "select /* order by asc */ 1 from t order by a asc",
	}, {
		input: "select /* order by desc */ 1 from t order by a desc",
	}, {
		input: "select /* order by null */ 1 from t order by null",
	}, {
		input: "select /* limit a */ 1 from t limit a",
	}, {
		input: "select /* limit a,b */ 1 from t limit a, b",
	}, {
		input:  "select /* binary unary */ a- -b from t",
		output: "select /* binary unary */ a - -b from t",
	}, {
		input: "select /* - - */ - -b from t",
	}, {
		input: "select /* binary binary */ binary  binary b from t",
	}, {
		input: "select /* binary ~ */ binary  ~b from t",
	}, {
		input: "select /* ~ binary */ ~ binary b from t",
	}, {
		input: "select /* interval */ adddate('2008-01-02', interval 31 day) from t",
	}, {
		input: "select /* interval keyword */ adddate('2008-01-02', interval 1 year) from t",
	}, {
		input: "select /* dual */ 1 from dual",
	}, {
		input:  "select /* Dual */ 1 from Dual",
		output: "select /* Dual */ 1 from dual",
	}, {
		input:  "select /* DUAL */ 1 from Dual",
		output: "select /* DUAL */ 1 from dual",
	}, {
		input: "select /* column as bool in where */ a from t where b",
	}, {
		input: "select /* OR of columns in where */ * from t where a or b",
	}, {
		input: "select /* OR of mixed columns in where */ * from t where a = 5 or b and c is not null",
	}, {
		input: "select /* OR in select columns */ (a or b) from t where c = 5",
	}, {
		input: "select /* bool as select value */ a, true from t",
	}, {
		input: "select /* bool column in ON clause */ * from t join s on t.id = s.id and s.foo where t.bar",
	}, {
		input: "select /* bool in order by */ * from t order by a is null or b asc",
	}, {
		input: "select /* string in case statement */ if(max(case a when 'foo' then 1 else 0 end) = 1, 'foo', 'bar') as foobar from t",
	}, {
		input:  "/*!show databases*/",
		output: "show databases",
	}, {
		input:  "select /*!40101 * from*/ t",
		output: "select * from t",
	}, {
		input:  "select /*! * from*/ t",
		output: "select * from t",
	}, {
		input:  "select /*!* from*/ t",
		output: "select * from t",
	}, {
		input:  "select /*!401011 from*/ t",
		output: "select 1 from t",
	}, {
		input: "select /* dual */ 1 from dual",
	}, {
		input: "insert /* simple */ into a values (1)",
	}, {
		input: "insert /* a.b */ into a.b values (1)",
	}, {
		input: "insert /* multi-value */ into a values (1, 2)",
	}, {
		input: "insert /* multi-value list */ into a values (1, 2), (3, 4)",
	}, {
		input: "insert /* no values */ into a values ()",
	}, {
		input:  "insert /* set */ into a set a = 1, b = 2",
		output: "insert /* set */ into a(a, b) values (1, 2)",
	}, {
		input:  "insert /* set default */ into a set a = default, b = 2",
		output: "insert /* set default */ into a(a, b) values (default, 2)",
	}, {
		input: "insert /* value expression list */ into a values (a + 1, 2 * 3)",
	}, {
		input: "insert /* default */ into a values (default, 2 * 3)",
	}, {
		input: "insert /* column list */ into a(a, b) values (1, 2)",
	}, {
		input: "insert into a(a, b) values (1, ifnull(null, default(b)))",
	}, {
		input: "insert /* qualified column list */ into a(a, b) values (1, 2)",
	}, {
		input:  "insert /* qualified columns */ into t (t.a, t.b) values (1, 2)",
		output: "insert /* qualified columns */ into t(a, b) values (1, 2)",
	}, {
		input: "insert /* select */ into a select b, c from d",
	}, {
		input:  "insert /* no cols & paren select */ into a(select * from t)",
		output: "insert /* no cols & paren select */ into a select * from t",
	}, {
		input:  "insert /* cols & paren select */ into a(a,b,c) (select * from t)",
		output: "insert /* cols & paren select */ into a(a, b, c) select * from t",
	}, {
		input: "insert /* cols & union with paren select */ into a(b, c) (select d, e from f) union (select g from h)",
	}, {
		input: "insert /* on duplicate */ into a values (1, 2) on duplicate key update b = func(a), c = d",
	}, {
		input: "insert /* bool in insert value */ into a values (1, true, false)",
	}, {
		input: "insert /* bool in on duplicate */ into a values (1, 2) on duplicate key update b = false, c = d",
	}, {
		input: "insert /* bool in on duplicate */ into a values (1, 2, 3) on duplicate key update b = values(b), c = d",
	}, {
		input: "insert /* bool in on duplicate */ into a values (1, 2, 3) on duplicate key update b = values(a.b), c = d",
	}, {
		input: "insert /* bool expression on duplicate */ into a values (1, 2) on duplicate key update b = func(a), c = a > d",
	}, {
		input: "values row(1)",
	}, {
		input:  "values (1), (2, 3)",
		output: "values row(1), row(2, 3)",
	}, {
		input:  "VALUES ROW(1),ROW(2, 3)",
		output: "values row(1), row(2, 3)",
	}, {
		input:  "with c as (select 1) insert into t select * from c",
		output: "with c as (select 1 from dual) insert into t select * from c",
	}, {
		input: "update /* simple */ a set b = 3",
	}, {
		input: "update /* a.b */ a.b set b = 3",
	}, {
		input: "update /* list */ a set b = 3, c = 4",
	}, {
		input: "update /* expression */ a set b = 3 + 4",
	}, {
		input: "update /* where */ a set b = 3 where a = b",
	}, {
		input: "update /* order */ a set b = 3 order by c desc",
	}, {
		input: "update /* limit */ a set b = 3 limit c",
	}, {
		input: "update /* bool in update */ a set b = true",
	}, {
		input: "update /* bool expr in update */ a set b = 5 > 2",
	}, {
		input: "update /* bool in update where */ a set b = 5 where c",
	}, {
		input: "update /* table qualifier */ a set a.b = 3",
	}, {
		input: "update /* table qualifier */ a set t.a.b = 3",
	}, {
		input:  "update /* table alias */ tt aa set aa.cc = 3",
		output: "update /* table alias */ tt as aa set aa.cc = 3",
	}, {
		input:  "update (select id from foo) subqalias set id = 4",
		output: "update (select id from foo) as subqalias set id = 4",
	}, {
		input:  "update foo f, bar b set f.id = b.id where b.name = 'test'",
		output: "update foo as f, bar as b set f.id = b.id where b.name = 'test'",
	}, {
		input:  "update foo f join bar b on f.name = b.name set f.id = b.id where b.name = 'test'",
		output: "update foo as f join bar as b on f.name = b.name set f.id = b.id where b.name = 'test'",
	}, {
		input:  "with c as (select 1 as id from dual) update t set a = 1 where id in (select id from c)",
		output: "with c as (select 1 as id from dual) update t set a = 1 where id in (select id from c)",
	}, {
		input: "delete /* simple */ from a",
	}, {
		input: "delete /* a.b */ from a.b",
	}, {
		input: "delete /* where */ from a where a = b",
	}, {
		input: "delete /* order */ from a order by b desc",
	}, {
		input: "delete /* limit */ from a limit b",
	}, {
		input: "delete a from a join b on a.id = b.id where b.name = 'test'",
	}, {
		input: "delete a, b from a, b where a.id = b.id and b.name = 'test'",
	}, {
		input:  "delete from a1, a2 using t1 as a1 inner join t2 as a2 where a1.id=a2.id",
		output: "delete a1, a2 from t1 as a1 join t2 as a2 where a1.id = a2.id",
	}, {
		input:  "with c as (select 1 as id from dual) delete from t where id in (select id from c)",
		output: "with c as (select 1 as id from dual) delete from t where id in (select id from c)",
	}, {
		input: "set /* simple */ a = 3",
	}, {
		input: "set #simple\n b = 4",
	}, {
		input: "set character_set_results = utf8",
	}, {
		input: "set @@session.autocommit = true",
	}, {
		input: "set @@session.`autocommit` = true",
	}, {
		input: "set @@session.\"autocommit\" = true",
	}, {
		input:  "set names utf8 collate foo",
		output: "set names 'utf8'",
	}, {
		input:  "set character set utf8",
		output: "set charset 'utf8'",
	}, {
		input:  "set character set 'utf8'",
		output: "set charset 'utf8'",
	}, {
		input:  "set character set \"utf8\"",
		output: "set charset 'utf8'",
	}, {
		input:  "set charset default",
		output: "set charset default",
	}, {
		input:  "set session wait_timeout = 3600",
		output: "set session wait_timeout = 3600",
	}, {
		input: "set /* list */ a = 3, b = 4",
	}, {
		input: "set /* mixed list */ a = 3, names 'utf8', charset 'ascii', b = 4",
	}, {
		input:  "set session transaction isolation level repeatable read",
		output: "set session tx_isolation = 'repeatable read'",
	}, {
		input:  "set global transaction isolation level repeatable read",
		output: "set global tx_isolation = 'repeatable read'",
	}, {
		input:  "set transaction isolation level repeatable read",
		output: "set tx_isolation = 'repeatable read'",
	}, {
		input:  "set transaction isolation level read committed",
		output: "set tx_isolation = 'read committed'",
	}, {
		input:  "set transaction isolation level read uncommitted",
		output: "set tx_isolation = 'read uncommitted'",
	}, {
		input:  "set transaction isolation level serializable",
		output: "set tx_isolation = 'serializable'",
	}, {
		input:  "set transaction read write",
		output: "set tx_read_only = 0",
	}, {
		input:  "set transaction read only",
		output: "set tx_read_only = 1",
	}, {
		input: "set sql_safe_updates = 0",
	}, {
		input: "set sql_safe_updates = 1",
	}, {
		input:  "alter table a add unique key foo (column1)",
		output: "alter table a add unique key foo (column1)",
	}, {
		input:  "alter table a alter foo",
		output: "alter table a",
	}, {
		input:  "alter table a change foo",
		output: "alter table a",
	}, {
		input:  "alter table a disable foo",
		output: "alter table a",
	}, {
		input:  "alter table a enable foo",
		output: "alter table a",
	}, {
		input:  "alter table a order foo",
		output: "alter table a",
	}, {
		input:  "alter table a default foo",
		output: "alter table a",
	}, {
		input:  "alter table a discard foo",
		output: "alter table a",
	}, {
		input:  "alter table a import foo",
		output: "alter table a",
	}, {
		input:  "alter table a rename b",
		output: "rename table a to b",
	}, {
		input:  "alter table `By` rename `bY`",
		output: "rename table `By` to `bY`",
	}, {
		input:  "alter table a rename to b",
		output: "rename table a to b",
	}, {
		input:  "alter table a rename as b",
		output: "rename table a to b",
	}, {
		input:  "alter table a rename index foo to bar",
		output: "alter table a",
	}, {
		input:  "alter table a rename key foo to bar",
		output: "alter table a",
	}, {
		input:  "alter table e auto_increment = 20",
		output: "alter table e",
	}, {
		input:  "alter table e character set = 'ascii'",
		output: "alter table e",
	}, {
		input:  "alter table e default character set = 'ascii'",
		output: "alter table e",
	}, {
		input:  "alter table e comment = 'hello'",
		output: "alter table e",
	}, {
		input:  "alter table a reorganize partition b into (partition c values less than (?), partition d values less than (maxvalue))",
		output: "alter table a reorganize partition b into (partition c values less than (:v1), partition d values less than (maxvalue))",
	}, {
		input:  "alter table a partition by range (id) (partition p0 values less than (10), partition p1 values less than (maxvalue))",
		output: "alter table a",
	}, {
		input:  "alter table a add column id int",
		output: "alter table a",
	}, {
		input:  "alter table a add index idx (id)",
		output: "alter table a add index idx (id)",
	}, {
		input:  "alter table a add unique key uk_id (id)",
		output: "alter table a add unique key uk_id (id)",
	}, {
		input:  "alter table a add unique uk_id (id)",
		output: "alter table a add unique uk_id (id)",
	}, {
		input:  "alter table a add unique index uk_id (id)",
		output: "alter table a add unique index uk_id (id)",
	}, {
		input:  "alter table a add unique (id)",
		output: "alter table a add unique (id)",
	}, {
		input:  "alter table a add unique key (id)",
		output: "alter table a add unique key (id)",
	}, {
		input:  "alter table a add unique index (id)",
		output: "alter table a add unique index (id)",
	}, {
		input:  "alter table a add key (id)",
		output: "alter table a add key (id)",
	}, {
		input:  "alter table a add unique key (a, b)",
		output: "alter table a add unique key (a, b)",
	}, {
		input:  "alter table a add constraint uk_id unique key (id)",
		output: "alter table a add unique key uk_id (id)",
	}, {
		input:  "alter table a add constraint uk_id unique index (id)",
		output: "alter table a add unique index uk_id (id)",
	}, {
		input:  "alter table a add fulltext index idx_ft (id)",
		output: "alter table a add fulltext index idx_ft (id)",
	}, {
		input:  "alter table a add spatial index idx_sp (id)",
		output: "alter table a add spatial index idx_sp (id)",
	}, {
		input:  "alter table a add check (id > 0)",
		output: "alter table a add check (id > 0)",
	}, {
		input:  "alter table a add constraint id_positive check (id > 0)",
		output: "alter table a add constraint id_positive check (id > 0)",
	}, {
		input: "alter table a add (id2 int, key by_id2 (id2), constraint id2_positive check (id2 > 0))",
		output: "alter table a add (\n" +
			"\tid2 int,\n" +
			"\tkey by_id2 (id2),\n" +
			"\tconstraint id2_positive check (id2 > 0)\n" +
			")",
	}, {
		input:  "alter table a drop primary key",
		output: "alter table a",
	}, {
		input: "create table a",
	}, {
		input: "create temporary table a",
	}, {
		input:  "create table a as select 1",
		output: "create table a as select 1 from dual",
	}, {
		input:  "create table if not exists a as select 1",
		output: "create table if not exists a as select 1 from dual",
	}, {
		input:  "create temporary table if not exists a as select 1",
		output: "create temporary table if not exists a as select 1 from dual",
	}, {
		input:  "create table a (\n\t`a` int\n)",
		output: "create table a (\n\ta int\n)",
	}, {
		input:  "create temporary table if not exists a (\n\t`a` int\n)",
		output: "create temporary table if not exists a (\n\ta int\n)",
	}, {
		input: "create table `by` (\n\t`by` char\n)",
	}, {
		input:  "create table t_fulltext (\n\tc1 text,\n\tfulltext key idx_ft (c1)\n)",
		output: "create table t_fulltext (\n\tc1 text,\n\tfulltext key idx_ft (c1)\n)",
	}, {
		input:  "create table t_spatial (\n\tc1 geometry,\n\tspatial key idx_sp (c1)\n)",
		output: "create table t_spatial (\n\tc1 geometry,\n\tspatial key idx_sp (c1)\n)",
	}, {
		input:  "create table if not exists a (\n\t`a` int\n)",
		output: "create table if not exists a (\n\ta int\n)",
	}, {
		input:  "create table a ignore me this is garbage",
		output: "create table a",
	}, {
		input:  "create table a (a int, b char, c garbage)",
		output: "create table a",
	}, {
		input:  "create index a on b",
		output: "alter table b",
	}, {
		input:  "create unique index a on b",
		output: "alter table b",
	}, {
		input:  "create unique index a using foo on b",
		output: "alter table b",
	}, {
		input:  "create fulltext index a using foo on b",
		output: "alter table b",
	}, {
		input:  "create spatial index a using foo on b",
		output: "alter table b",
	}, {
		input:  "create view a as select 1",
		output: "create table a as select 1 from dual",
	}, {
		input:  "create or replace view a as select 1",
		output: "create table a as select 1 from dual",
	}, {
		input:  "alter view a as select 1",
		output: "alter table a as select 1 from dual",
	}, {
		input:  "drop view a",
		output: "drop table a",
	}, {
		input:  "drop table a",
		output: "drop table a",
	}, {
		input:  "drop table if exists a",
		output: "drop table if exists a",
	}, {
		input:  "drop view if exists a",
		output: "drop table if exists a",
	}, {
		input:  "drop index b on a",
		output: "alter table a",
	}, {
		input:  "analyze table a",
		output: "alter table a",
	}, {
		input:  "show binary logs",
		output: "show binary logs",
	}, {
		input:  "show binlog events",
		output: "show binlog",
	}, {
		input:  "show character set",
		output: "show character set",
	}, {
		input:  "show character set like '%foo'",
		output: "show character set",
	}, {
		input:  "show collation",
		output: "show collation",
	}, {
		input:  "show create database d",
		output: "show create database",
	}, {
		input:  "show create event e",
		output: "show create event",
	}, {
		input:  "show create function f",
		output: "show create function",
	}, {
		input:  "show create procedure p",
		output: "show create procedure",
	}, {
		input:  "show create table t",
		output: "show create table",
	}, {
		input:  "show create trigger t",
		output: "show create trigger",
	}, {
		input:  "show create user u",
		output: "show create user",
	}, {
		input:  "show create view v",
		output: "show create view",
	}, {
		input:  "show databases",
		output: "show databases",
	}, {
		input:  "show engine INNODB",
		output: "show engine",
	}, {
		input:  "show engines",
		output: "show engines",
	}, {
		input:  "show storage engines",
		output: "show storage",
	}, {
		input:  "show errors",
		output: "show errors",
	}, {
		input:  "show events",
		output: "show events",
	}, {
		input:  "show function code func",
		output: "show function",
	}, {
		input:  "show function status",
		output: "show function",
	}, {
		input:  "show grants for 'root@localhost'",
		output: "show grants",
	}, {
		input:  "show index from table",
		output: "show index",
	}, {
		input:  "show indexes from table",
		output: "show indexes",
	}, {
		input:  "show keys from table",
		output: "show keys",
	}, {
		input:  "show master status",
		output: "show master",
	}, {
		input:  "show open tables",
		output: "show open",
	}, {
		input:  "show plugins",
		output: "show plugins",
	}, {
		input:  "show privileges",
		output: "show privileges",
	}, {
		input:  "show procedure code p",
		output: "show procedure",
	}, {
		input:  "show procedure status",
		output: "show procedure",
	}, {
		input:  "show processlist",
		output: "show processlist",
	}, {
		input:  "show full processlist",
		output: "show processlist",
	}, {
		input:  "show profile cpu for query 1",
		output: "show profile",
	}, {
		input:  "show profiles",
		output: "show profiles",
	}, {
		input:  "show relaylog events",
		output: "show relaylog",
	}, {
		input:  "show slave hosts",
		output: "show slave",
	}, {
		input:  "show slave status",
		output: "show slave",
	}, {
		input:  "show status",
		output: "show status",
	}, {
		input:  "show global status",
		output: "show global status",
	}, {
		input:  "show session status",
		output: "show session status",
	}, {
		input:  "show table status",
		output: "show table",
	}, {
		input: "show tables",
	}, {
		input: "show tables like '%keyspace%'",
	}, {
		input: "show tables where 1 = 0",
	}, {
		input: "show tables from a",
	}, {
		input: "show tables from a where 1 = 0",
	}, {
		input: "show tables from a like '%keyspace%'",
	}, {
		input: "show full tables",
	}, {
		input: "show full tables from a",
	}, {
		input:  "show full tables in a",
		output: "show full tables from a",
	}, {
		input: "show full tables from a like '%keyspace%'",
	}, {
		input: "show full tables from a where 1 = 0",
	}, {
		input: "show full tables like '%keyspace%'",
	}, {
		input: "show full tables where 1 = 0",
	}, {
		input:  "show triggers",
		output: "show triggers",
	}, {
		input:  "show variables",
		output: "show variables",
	}, {
		input:  "show global variables",
		output: "show global variables",
	}, {
		input:  "show session variables",
		output: "show session variables",
	}, {
		input:  "show warnings",
		output: "show warnings",
	}, {
		input:  "use db",
		output: "use db",
	}, {
		input:  "use duplicate",
		output: "use `duplicate`",
	}, {
		input:  "use `ks:-80@master`",
		output: "use `ks:-80@master`",
	}, {
		input:  "describe foobar",
		output: "otherread",
	}, {
		input:  "desc foobar",
		output: "otherread",
	}, {
		input:  "explain foobar",
		output: "otherread",
	}, {
		input:  "truncate table foo",
		output: "truncate table foo",
	}, {
		input:  "truncate foo",
		output: "truncate table foo",
	}, {
		input:  "repair table foo",
		output: "otheradmin",
	}, {
		input:  "optimize table foo",
		output: "otheradmin",
	}, {
		input: "select /* EQ true */ 1 from t where a = true",
	}, {
		input: "select /* EQ false */ 1 from t where a = false",
	}, {
		input: "select /* NE true */ 1 from t where a != true",
	}, {
		input: "select /* NE false */ 1 from t where a != false",
	}, {
		input: "select /* LT true */ 1 from t where a < true",
	}, {
		input: "select /* LT false */ 1 from t where a < false",
	}, {
		input: "select /* GT true */ 1 from t where a > true",
	}, {
		input: "select /* GT false */ 1 from t where a > false",
	}, {
		input: "select /* LE true */ 1 from t where a <= true",
	}, {
		input: "select /* LE false */ 1 from t where a <= false",
	}, {
		input: "select /* GE true */ 1 from t where a >= true",
	}, {
		input: "select /* GE false */ 1 from t where a >= false",
	}, {
		input:  "select * from t order by a collate utf8_general_ci",
		output: "select * from t order by a collate utf8_general_ci asc",
	}, {
		input: "select k collate latin1_german2_ci as k1 from t1 order by k1 asc",
	}, {
		input: "select * from t group by a collate utf8_general_ci",
	}, {
		input: "select MAX(k collate latin1_german2_ci) from t1",
	}, {
		input: "select distinct k collate latin1_german2_ci from t1",
	}, {
		input: "select * from t1 where 'Müller' collate latin1_german2_ci = k",
	}, {
		input: "select * from t1 where k like 'Müller' collate latin1_german2_ci",
	}, {
		input: "select k from t1 group by k having k = 'Müller' collate latin1_german2_ci",
	}, {
		input: "select k from t1 join t2 order by a collate latin1_german2_ci asc, b collate latin1_german2_ci asc",
	}, {
		input:  "select k collate 'latin1_german2_ci' as k1 from t1 order by k1 asc",
		output: "select k collate latin1_german2_ci as k1 from t1 order by k1 asc",
	}, {
		input:  "select /* drop trailing semicolon */ 1 from dual;",
		output: "select /* drop trailing semicolon */ 1 from dual",
	}, {
		input: "select /* cache directive */ sql_no_cache 'foo' from t",
	}, {
		input: "select binary 'a' = 'A' from t",
	}, {
		input: "select 1 from t where foo = _binary 'bar'",
	}, {
		input:  "select 1 from t where foo = _binary'bar'",
		output: "select 1 from t where foo = _binary 'bar'",
	}, {
		input: "select match(a) against ('foo') from t",
	}, {
		input: "select match(a1, a2) against ('foo' in natural language mode with query expansion) from t",
	}, {
		input: "select title from video as v where match(v.title, v.tag) against ('DEMO' in boolean mode)",
	}, {
		input: "select name, group_concat(score) from t group by name",
	}, {
		input: "select name, group_concat(distinct id, score order by id desc separator ':') from t group by name",
	}, {
		input: "select * from t partition (p0)",
	}, {
		input: "select * from t partition (p0, p1)",
	}, {
		input: "select e.id, s.city from employees as e join stores partition (p1) as s on e.store_id = s.id",
	}, {
		input: "select truncate(120.3333, 2) from dual",
	}, {
		input: "update t partition (p0) set a = 1",
	}, {
		input: "insert into t partition (p0) values (1, 'asdf')",
	}, {
		input: "insert into t1 select * from t2 partition (p0)",
	}, {
		input: "replace into t partition (p0) values (1, 'asdf')",
	}, {
		input: "delete from t partition (p0) where a = 1",
	}, {
		input: "begin",
	}, {
		input:  "start transaction",
		output: "begin",
	}, {
		input: "commit",
	}, {
		input: "rollback",
	}, {
		input: "grant select on appdb.users to 'app'@'%'",
	}, {
		input: "grant select, insert on appdb.* to 'app'@'localhost', 'readonly'@'%'",
	}, {
		input:  "grant all privileges on *.* to 'root'@'localhost' with grant option",
		output: "grant all on *.* to 'root'@'localhost' with grant option",
	}, {
		input: "revoke select on appdb.users from 'app'@'%'",
	}, {
		input: "revoke grant option for select, insert on appdb.* from 'app'@'localhost'",
	}, {
		input: "create database test_db",
	}, {
		input:  "create schema test_db",
		output: "create database test_db",
	}, {
		input:  "create database if not exists test_db",
		output: "create database test_db",
	}, {
		input:  "CREATE DATABASE /*!32312 IF NOT EXISTS*/ `test_db` /*!40100 DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci */",
		output: "create database test_db",
	}, {
		input:  "CREATE DATABASE `test_db` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci",
		output: "create database test_db",
	}, {
		input:  "CREATE SCHEMA /*!32312 IF NOT EXISTS*/ `test_db` /*!40100 DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci */",
		output: "create database test_db",
	}, {
		input: "drop database test_db",
	}, {
		input:  "drop schema test_db",
		output: "drop database test_db",
	}, {
		input:  "drop database if exists test_db",
		output: "drop database test_db",
	}}
)

func TestValid(t *testing.T) {
	for _, tcase := range validSQL {
		if tcase.output == "" {
			tcase.output = tcase.input
		}
		tree, err := Parse(tcase.input)
		if err != nil {
			t.Errorf("Parse(%q) err: %v, want nil", tcase.input, err)
			continue
		}
		out := String(tree)
		if out != tcase.output {
			t.Errorf("Parse(%q) = %q, want: %q", tcase.input, out, tcase.output)
		}
		// This test just exercises the tree walking functionality.
		// There's no way automated way to verify that a node calls
		// all its children. But we can examine code coverage and
		// ensure that all walkSubtree functions were called.
		Walk(func(node SQLNode) (bool, error) {
			return true, nil
		}, tree)
	}
}

func TestCTEInvalid(t *testing.T) {
	invalidSQL := []string{
		"with cte as select 1 select * from cte",
		"with recursive select 1",
		"with cte as (select 1) with d as (select 2) select * from cte",
	}
	for _, sql := range invalidSQL {
		if _, err := Parse(sql); err == nil {
			t.Errorf("Parse(%q) err: nil, want non-nil", sql)
		}
	}
}

func TestWindowInvalid(t *testing.T) {
	invalidSQL := []string{
		"select row_number() over from t",
		"select row_number() over () from t",
		"select row_number() over (partition dept) from t",
		"select row_number() over (rows between 1 preceding and) from t",
		"select 1 from t window w (partition by a)",
	}
	for _, sql := range invalidSQL {
		if _, err := Parse(sql); err == nil {
			t.Errorf("Parse(%q) err: nil, want non-nil", sql)
		}
	}
}

func TestSelectLockInvalid(t *testing.T) {
	invalidSQL := []string{
		"select 1 from t for update nowait skip locked",
		"select 1 from t for update skip locked nowait",
		"select 1 from t for share nowait skip locked",
		"select 1 from t for share skip locked nowait",
		"select 1 from t for share lock in share mode",
		"select 1 from t lock in share mode nowait",
	}
	for _, sql := range invalidSQL {
		if _, err := Parse(sql); err == nil {
			t.Errorf("Parse(%q) err: nil, want non-nil", sql)
		}
	}
}

func TestGrantRevokeInvalid(t *testing.T) {
	invalidSQL := []string{
		"grant on appdb.users to 'app'@'%'",
		"grant select on appdb.users 'app'@'%'",
		"revoke select on appdb.users to 'app'@'%'",
		"revoke grant option select on appdb.users from 'app'@'%'",
	}
	for _, sql := range invalidSQL {
		if _, err := Parse(sql); err == nil {
			t.Errorf("Parse(%q) err: nil, want non-nil", sql)
		}
	}
}

func TestJSONTableInvalid(t *testing.T) {
	invalidSQL := []string{
		// ON EMPTY / ON ERROR clauses are intentionally unsupported in the initial JSON_TABLE grammar.
		"select * from json_table(doc, '$' columns (id int path '$.id' null on empty)) as jt",
		"select * from json_table(doc, '$' columns (id int path '$.id' null on error)) as jt",
	}
	for _, sql := range invalidSQL {
		if _, err := Parse(sql); err == nil {
			t.Errorf("Parse(%q) err: nil, want non-nil", sql)
		}
	}
}

func TestValuesInvalid(t *testing.T) {
	invalidSQL := []string{
		"values row()",
	}
	for _, sql := range invalidSQL {
		if _, err := Parse(sql); err == nil {
			t.Errorf("Parse(%q) err: nil, want non-nil", sql)
		}
	}
}

func TestMySQL80RemovedSyntaxInvalid(t *testing.T) {
	invalidSQL := []string{
		"select next value for t",
		"select next 10 values from t",
		"stream * from t",
		"stream /* comment */ * from t",
		"select :a from t",
		"select :a1 from t",
		"select :a.b from t",
		"select * from t where a in ::list",
		"create vindex hash_vdx using hash",
		"alter table a add vindex hash (id)",
		"alter table a drop vindex hash",
		"show vindexes",
		"show vindexes on t",
		"show vitess_keyspaces",
		"show vitess_shards",
		"show vitess_tablets",
		"show vschema_tables",
		"show foobar",
		"show foobar like select * from table where syntax is 'ignored'",
		"create view a",
		"alter view a",
		"repair foo",
		"optimize foo",
		"set tx_isolation = 'repeatable read'",
		"set tx_read_only = 1",
		"set session tx_isolation = 'repeatable read'",
		"set session tx_read_only = 1",
		"set @@session.'autocommit' = true",
		"alter table tbl_incomplete add constraint",
		"alter table tbl_incomplete add foreign key",
		"alter table tbl_incomplete add primary",
		"alter table tbl_incomplete add id",
		"alter table tbl_incomplete modify col_a",
		"alter table tbl_incomplete modify column col_a",
		"alter table tbl_incomplete drop foreign key",
		"alter table tbl_incomplete drop constraint",
		"alter table tbl_incomplete rename key",
	}
	for _, sql := range invalidSQL {
		if _, err := Parse(sql); err == nil {
			t.Errorf("Parse(%q) err: nil, want non-nil", sql)
		}
	}
}

func TestCaseSensitivity(t *testing.T) {
	validSQL := []struct {
		input  string
		output string
	}{{
		input:  "create table A (\n\t`B` int\n)",
		output: "create table A (\n\tB int\n)",
	}, {
		input:  "create index b on A",
		output: "alter table A",
	}, {
		input:  "alter table A foo",
		output: "alter table A",
	}, {
		input:  "alter table A convert",
		output: "alter table A",
	}, {
		// View names get lower-cased.
		input:  "alter view A as select 1",
		output: "alter table a as select 1 from dual",
	}, {
		input:  "alter table A rename to B",
		output: "rename table A to B",
	}, {
		input: "rename table A to B",
	}, {
		input:  "drop table B",
		output: "drop table B",
	}, {
		input:  "drop table if exists B",
		output: "drop table if exists B",
	}, {
		input:  "drop index b on A",
		output: "alter table A",
	}, {
		input: "select a from B",
	}, {
		input: "select A as B from C",
	}, {
		input: "select B.* from c",
	}, {
		input: "select B.A from c",
	}, {
		input: "select * from B as C",
	}, {
		input: "select * from A.B",
	}, {
		input: "update A set b = 1",
	}, {
		input: "update A.B set b = 1",
	}, {
		input: "select A() from b",
	}, {
		input: "select A(B, C) from b",
	}, {
		input: "select A(distinct B, C) from b",
	}, {
		// IF is an exception. It's always lower-cased.
		input:  "select IF(B, C) from b",
		output: "select if(B, C) from b",
	}, {
		input: "select * from b use index (A)",
	}, {
		input: "insert into A(A, B) values (1, 2)",
	}, {
		input:  "CREATE TABLE A (\n\t`A` int\n)",
		output: "create table A (\n\tA int\n)",
	}, {
		input:  "create view A as select 1",
		output: "create table a as select 1 from dual",
	}, {
		input:  "alter view A as select 1",
		output: "alter table a as select 1 from dual",
	}, {
		input:  "drop view A",
		output: "drop table a",
	}, {
		input:  "drop view if exists A",
		output: "drop table if exists a",
	}, {
		input:  "select /* lock in SHARE MODE */ 1 from t lock in SHARE MODE",
		output: "select /* lock in SHARE MODE */ 1 from t lock in share mode",
	}, {
		input:  "select /* FOR SHARE NOWAIT */ 1 from t FOR SHARE NOWAIT",
		output: "select /* FOR SHARE NOWAIT */ 1 from t for share nowait",
	}, {
		input:  "select /* FOR UPDATE SKIP LOCKED */ 1 from t FOR UPDATE SKIP LOCKED",
		output: "select /* FOR UPDATE SKIP LOCKED */ 1 from t for update skip locked",
	}, {
		input: "select /* use */ 1 from t1 use index (A) where b = 1",
	}}
	for _, tcase := range validSQL {
		if tcase.output == "" {
			tcase.output = tcase.input
		}
		tree, err := Parse(tcase.input)
		if err != nil {
			t.Errorf("input: %s, err: %v", tcase.input, err)
			continue
		}
		out := String(tree)
		if out != tcase.output {
			t.Errorf("out: %s, want %s", out, tcase.output)
		}
	}
}

func TestKeywords(t *testing.T) {
	validSQL := []struct {
		input  string
		output string
	}{{
		input:  "select current_timestamp",
		output: "select current_timestamp() from dual",
	}, {
		input: "update t set a = current_timestamp()",
	}, {
		input: "select current_timestamp(6) from t",
	}, {
		input: "select current_time(6) from t",
	}, {
		input: "select localtimestamp(6) from t",
	}, {
		input: "select localtime(6) from t",
	}, {
		input: "select utc_time(6), utc_timestamp(6) from t",
	}, {
		input: "select now(6), curtime(6), sysdate(6) from t",
	}, {
		input:  "select a, current_date from t",
		output: "select a, current_date() from t",
	}, {
		input:  "insert into t(a, b) values (current_date, current_date())",
		output: "insert into t(a, b) values (current_date(), current_date())",
	}, {
		input: "select * from t where a > utc_timestmp()",
	}, {
		input:  "update t set b = utc_timestamp + 5",
		output: "update t set b = utc_timestamp() + 5",
	}, {
		input:  "select utc_time, utc_date",
		output: "select utc_time(), utc_date() from dual",
	}, {
		input:  "select 1 from dual where localtime > utc_time",
		output: "select 1 from dual where localtime() > utc_time()",
	}, {
		input:  "update t set a = localtimestamp(), b = utc_timestamp",
		output: "update t set a = localtimestamp(), b = utc_timestamp()",
	}, {
		input: "insert into t(a) values (unix_timestamp)",
	}, {
		input: "select replace(a, 'foo', 'bar') from t",
	}, {
		input: "update t set a = replace('1234', '2', '1')",
	}, {
		input: "insert into t(a, b) values ('foo', 'bar') on duplicate key update a = replace(hex('foo'), 'f', 'b')",
	}, {
		input: "update t set a = left('1234', 3)",
	}, {
		input: "select left(a, 5) from t",
	}, {
		input: "update t set d = adddate(date('2003-12-31 01:02:03'), interval 5 days)",
	}, {
		input: "insert into t(a, b) values (left('foo', 1), 'b')",
	}, {
		input: "insert /* qualified function */ into t(a, b) values (test.PI(), 'b')",
	}, {
		input:  "select /* keyword in qualified id */ * from t join z on t.key = z.key",
		output: "select /* keyword in qualified id */ * from t join z on t.`key` = z.`key`",
	}, {
		input:  "select /* non-reserved keywords as unqualified cols */ date, view, offset from t",
		output: "select /* non-reserved keywords as unqualified cols */ `date`, `view`, `offset` from t",
	}, {
		input:  "select /* share and mode as cols */ share, mode from t where share = 'foo'",
		output: "select /* share and mode as cols */ `share`, `mode` from t where `share` = 'foo'",
	}, {
		input:  "select /* unused keywords as cols */ write, varying from t where trailing = 'foo'",
		output: "select /* unused keywords as cols */ `write`, `varying` from t where `trailing` = 'foo'",
	}, {
		input:  "select status from t",
		output: "select `status` from t",
	}, {
		input:  "select variables from t",
		output: "select `variables` from t",
	}}

	for _, tcase := range validSQL {
		if tcase.output == "" {
			tcase.output = tcase.input
		}
		tree, err := Parse(tcase.input)
		if err != nil {
			t.Errorf("input: %s, err: %v", tcase.input, err)
			continue
		}
		out := String(tree)
		if out != tcase.output {
			t.Errorf("out: %s, want %s", out, tcase.output)
		}
	}
}

func TestConvert(t *testing.T) {
	validSQL := []struct {
		input  string
		output string
	}{{
		input:  "select cast('abc' as date) from t",
		output: "select convert('abc', date) from t",
	}, {
		input: "select convert('abc', binary(4)) from t",
	}, {
		input: "select convert('abc', binary) from t",
	}, {
		input: "select convert('abc', char character set binary) from t",
	}, {
		input: "select convert('abc', char(4) ascii) from t",
	}, {
		input: "select convert('abc', char unicode) from t",
	}, {
		input: "select convert('abc', char(4)) from t",
	}, {
		input: "select convert('abc', char) from t",
	}, {
		input: "select convert('abc', nchar(4)) from t",
	}, {
		input: "select convert('abc', nchar) from t",
	}, {
		input: "select convert('abc', signed) from t",
	}, {
		input:  "select convert('abc', signed integer) from t",
		output: "select convert('abc', signed) from t",
	}, {
		input: "select convert('abc', unsigned) from t",
	}, {
		input:  "select convert('abc', unsigned integer) from t",
		output: "select convert('abc', unsigned) from t",
	}, {
		input: "select convert('abc', decimal(3, 4)) from t",
	}, {
		input: "select convert('abc', decimal(4)) from t",
	}, {
		input: "select convert('abc', decimal) from t",
	}, {
		input: "select convert('abc', date) from t",
	}, {
		input: "select convert('abc', time(4)) from t",
	}, {
		input: "select convert('abc', time) from t",
	}, {
		input: "select convert('abc', datetime(9)) from t",
	}, {
		input: "select convert('abc', datetime) from t",
	}, {
		input: "select convert('abc', json) from t",
	}, {
		input: "select convert('abc' using ascii) from t",
	}}

	for _, tcase := range validSQL {
		if tcase.output == "" {
			tcase.output = tcase.input
		}
		tree, err := Parse(tcase.input)
		if err != nil {
			t.Errorf("input: %s, err: %v", tcase.input, err)
			continue
		}
		out := String(tree)
		if out != tcase.output {
			t.Errorf("out: %s, want %s", out, tcase.output)
		}
	}

	invalidSQL := []struct {
		input  string
		output string
	}{{
		input:  "select convert('abc' as date) from t",
		output: "syntax error at position 24 near 'as'",
	}, {
		input:  "select convert from t",
		output: "syntax error at position 20 near 'from'",
	}, {
		input:  "select cast('foo', decimal) from t",
		output: "syntax error at position 19",
	}, {
		input:  "select convert('abc', datetime(4+9)) from t",
		output: "syntax error at position 34",
	}, {
		input:  "select convert('abc', decimal(4+9)) from t",
		output: "syntax error at position 33",
	}}

	for _, tcase := range invalidSQL {
		_, err := Parse(tcase.input)
		if err == nil || err.Error() != tcase.output {
			t.Errorf("%s: %v, want %s", tcase.input, err, tcase.output)
		}
	}
}

func TestSubStr(t *testing.T) {

	validSQL := []struct {
		input  string
		output string
	}{{
		input: "select substr(a, 1) from t",
	}, {
		input: "select substr(a, 1, 6) from t",
	}, {
		input:  "select substring(a, 1) from t",
		output: "select substr(a, 1) from t",
	}, {
		input:  "select substring(a, 1, 6) from t",
		output: "select substr(a, 1, 6) from t",
	}, {
		input:  "select substr(a from 1 for 6) from t",
		output: "select substr(a, 1, 6) from t",
	}, {
		input:  "select substring(a from 1 for 6) from t",
		output: "select substr(a, 1, 6) from t",
	}}

	for _, tcase := range validSQL {
		if tcase.output == "" {
			tcase.output = tcase.input
		}
		tree, err := Parse(tcase.input)
		if err != nil {
			t.Errorf("input: %s, err: %v", tcase.input, err)
			continue
		}
		out := String(tree)
		if out != tcase.output {
			t.Errorf("out: %s, want %s", out, tcase.output)
		}
	}
}

func TestCreateTable(t *testing.T) {
	validSQL := []string{
		"create table t as select 1 from dual",
		"create table t like src",
		"create table if not exists t like src",
		"create temporary table t like src",
		"create temporary table if not exists t like src",

		// test all the data types and options
		"create table t (\n" +
			"	col_bit bit,\n" +
			"	col_tinyint tinyint auto_increment,\n" +
			"	col_tinyint3 tinyint(3) unsigned,\n" +
			"	col_smallint smallint,\n" +
			"	col_smallint4 smallint(4) zerofill,\n" +
			"	col_mediumint mediumint,\n" +
			"	col_mediumint5 mediumint(5) unsigned not null,\n" +
			"	col_int int,\n" +
			"	col_int10 int(10) not null,\n" +
			"	col_integer integer comment 'this is an integer',\n" +
			"	col_bigint bigint,\n" +
			"	col_bigint10 bigint(10) zerofill not null default 10,\n" +
			"	col_real real,\n" +
			"	col_real2 real(1,2) not null default 1.23,\n" +
			"	col_double double,\n" +
			"	col_double2 double(3,4) not null default 1.23,\n" +
			"	col_float float,\n" +
			"	col_float2 float(3,4) not null default 1.23,\n" +
			"	col_decimal decimal,\n" +
			"	col_decimal2 decimal(2),\n" +
			"	col_decimal3 decimal(2,3),\n" +
			"	col_numeric numeric,\n" +
			"	col_numeric2 numeric(2),\n" +
			"	col_numeric3 numeric(2,3),\n" +
			"	col_date date,\n" +
			"	col_time time,\n" +
			"	col_timestamp timestamp,\n" +
			"	col_datetime datetime,\n" +
			"	col_year year,\n" +
			"	col_char char,\n" +
			"	col_char2 char(2),\n" +
			"	col_char3 char(3) character set ascii,\n" +
			"	col_char4 char(4) character set ascii collate ascii_bin,\n" +
			"	col_varchar varchar,\n" +
			"	col_varchar2 varchar(2),\n" +
			"	col_varchar3 varchar(3) character set ascii,\n" +
			"	col_varchar4 varchar(4) character set ascii collate ascii_bin,\n" +
			"	col_binary binary,\n" +
			"	col_varbinary varbinary(10),\n" +
			"	col_tinyblob tinyblob,\n" +
			"	col_blob blob,\n" +
			"	col_mediumblob mediumblob,\n" +
			"	col_longblob longblob,\n" +
			"	col_tinytext tinytext,\n" +
			"	col_text text,\n" +
			"	col_mediumtext mediumtext,\n" +
			"	col_longtext longtext,\n" +
			"	col_text text character set ascii collate ascii_bin,\n" +
			"	col_json json,\n" +
			"	col_enum enum('a', 'b', 'c', 'd'),\n" +
			"	col_enum2 enum('a', 'b', 'c', 'd') character set ascii,\n" +
			"	col_enum3 enum('a', 'b', 'c', 'd') collate ascii_bin,\n" +
			"	col_enum4 enum('a', 'b', 'c', 'd') character set ascii collate ascii_bin,\n" +
			"	col_set set('a', 'b', 'c', 'd'),\n" +
			"	col_set2 set('a', 'b', 'c', 'd') character set ascii,\n" +
			"	col_set3 set('a', 'b', 'c', 'd') collate ascii_bin,\n" +
			"	col_set4 set('a', 'b', 'c', 'd') character set ascii collate ascii_bin,\n" +
			"	col_geometry1 geometry,\n" +
			"	col_geometry2 geometry not null,\n" +
			"	col_point1 point,\n" +
			"	col_point2 point not null,\n" +
			"	col_linestring1 linestring,\n" +
			"	col_linestring2 linestring not null,\n" +
			"	col_polygon1 polygon,\n" +
			"	col_polygon2 polygon not null,\n" +
			"	col_geometrycollection1 geometrycollection,\n" +
			"	col_geometrycollection2 geometrycollection not null,\n" +
			"	col_multipoint1 multipoint,\n" +
			"	col_multipoint2 multipoint not null,\n" +
			"	col_multilinestring1 multilinestring,\n" +
			"	col_multilinestring2 multilinestring not null,\n" +
			"	col_multipolygon1 multipolygon,\n" +
			"	col_multipolygon2 multipolygon not null\n" +
			")",

		// test defaults
		"create table t (\n" +
			"	i1 int default 1,\n" +
			"	i2 int default null,\n" +
			"	f1 float default 1.23,\n" +
			"	s1 varchar default 'c',\n" +
			"	s2 varchar default 'this is a string',\n" +
			"	s3 varchar default null,\n" +
			"	s4 timestamp default current_timestamp,\n" +
			"	s7 timestamp default current_timestamp(),\n" +
			"	s6 timestamp(6) default current_timestamp(6),\n" +
			"	s5 bit(1) default B'0'\n" +
			")",

		// test key field options
		"create table t (\n" +
			"	id int auto_increment primary key,\n" +
			"	username varchar unique key,\n" +
			"	email varchar unique,\n" +
			"	full_name varchar key,\n" +
			"	time1 timestamp on update current_timestamp,\n" +
			"	time4 timestamp on update current_timestamp(),\n" +
			"	time3 timestamp(6) on update current_timestamp(6),\n" +
			"	time2 timestamp default current_timestamp on update current_timestamp\n" +
			")",
		// test generated columns
		"create table t (\n" +
			"	price int,\n" +
			"	qty int,\n" +
			"	total int generated always as (price * qty),\n" +
			"	total_virtual int generated always as (price * qty) virtual,\n" +
			"	total_stored int generated always as ((price * qty) + 1) stored key,\n" +
			"	total_comment int generated always as (ifnull(price, 0) * qty) stored comment 'computed'\n" +
			")",

		// test defining indexes separately
		"create table t (\n" +
			"	id int auto_increment,\n" +
			"	username varchar,\n" +
			"	email varchar,\n" +
			"	full_name varchar,\n" +
			"	geom point not null,\n" +
			"	status_nonkeyword varchar,\n" +
			"	primary key (id),\n" +
			"	spatial key geom (geom),\n" +
			"	unique key by_username (username),\n" +
			"	unique by_username2 (username),\n" +
			"	unique index by_username3 (username),\n" +
			"	index by_status (status_nonkeyword),\n" +
			"	key by_full_name (full_name)\n" +
			")",
		// test defining indexes separately without index names
		"create table t (\n" +
			"	col1 int,\n" +
			"	col2 int,\n" +
			"	col3 int,\n" +
			"	col4 int,\n" +
			"	unique (col4),\n" +
			"	unique key (col1),\n" +
			"	unique index (col2),\n" +
			"	key (col3),\n" +
			"	unique key (col1, col2)\n" +
			")",

		// test that indexes support USING <id>
		"create table t (\n" +
			"	id int auto_increment,\n" +
			"	username varchar,\n" +
			"	email varchar,\n" +
			"	full_name varchar,\n" +
			"	status_nonkeyword varchar,\n" +
			"	primary key (id) using BTREE,\n" +
			"	unique key by_username (username) using HASH,\n" +
			"	unique by_username2 (username) using OTHER,\n" +
			"	unique index by_username3 (username) using XYZ,\n" +
			"	index by_status (status_nonkeyword) using PDQ,\n" +
			"	key by_full_name (full_name) using OTHER\n" +
			")",
		// test other index options
		"create table t (\n" +
			"	id int auto_increment,\n" +
			"	username varchar,\n" +
			"	email varchar,\n" +
			"	primary key (id) comment 'hi',\n" +
			"	unique key by_username (username) key_block_size 8,\n" +
			"	unique index by_username4 (username) comment 'hi' using BTREE,\n" +
			"	unique index by_username4 (username) using BTREE key_block_size 4 comment 'hi'\n" +
			")",

		// multi-column indexes
		"create table t (\n" +
			"	id int auto_increment,\n" +
			"	username varchar,\n" +
			"	email varchar,\n" +
			"	full_name varchar,\n" +
			"	a int,\n" +
			"	b int,\n" +
			"	c int,\n" +
			"	primary key (id, username),\n" +
			"	unique key by_abc (a, b, c),\n" +
			"	key by_email (email(10), username)\n" +
			")",
		// table check constraints
		"create table t (\n" +
			"	id int,\n" +
			"	check (id > 0)\n" +
			")",
		"create table t (\n" +
			"	id int,\n" +
			"	constraint id_positive check (id > 0)\n" +
			")",
		// mixed column, index, and check constraint definitions
		"create table t (\n" +
			"	id int,\n" +
			"	key by_id (id),\n" +
			"	check (id > 0),\n" +
			"	constraint id_lt_100 check (id < 100)\n" +
			")",
		// foreign key constraints
		"create table t (\n" +
			"	id int,\n" +
			"	parent_id int,\n" +
			"	constraint fk_parent foreign key (parent_id) references parent (id)\n" +
			")",
		"create table t (\n" +
			"	id int,\n" +
			"	parent_id int,\n" +
			"	foreign key (parent_id) references parent (id) on delete set null on update cascade\n" +
			")",

		// table options
		"create table t (\n" +
			"	id int auto_increment\n" +
			") engine InnoDB,\n" +
			"  auto_increment 123,\n" +
			"  avg_row_length 1,\n" +
			"  default character set utf8mb4,\n" +
			"  character set latin1,\n" +
			"  checksum 0,\n" +
			"  default collate binary,\n" +
			"  collate ascii_bin,\n" +
			"  comment 'this is a comment',\n" +
			"  compression 'zlib',\n" +
			"  connection 'connect_string',\n" +
			"  data directory 'absolute path to directory',\n" +
			"  delay_key_write 1,\n" +
			"  encryption 'n',\n" +
			"  index directory 'absolute path to directory',\n" +
			"  insert_method no,\n" +
			"  key_block_size 1024,\n" +
			"  max_rows 100,\n" +
			"  min_rows 10,\n" +
			"  pack_keys 0,\n" +
			"  password 'sekret',\n" +
			"  row_format default,\n" +
			"  stats_auto_recalc default,\n" +
			"  stats_persistent 0,\n" +
			"  stats_sample_pages 1,\n" +
			"  tablespace tablespace_name storage disk,\n" +
			"  tablespace tablespace_name\n",
	}
	for _, sql := range validSQL {
		sql = strings.TrimSpace(sql)
		tree, err := ParseStrictDDL(sql)
		if err != nil {
			t.Errorf("input: %s, err: %v", sql, err)
			continue
		}
		got := String(tree.(*DDL))

		if sql != got {
			t.Errorf("want:\n%s\ngot:\n%s", sql, got)
		}
	}

	strictMySQLCommentDBDDL := []struct {
		input  string
		output string
	}{
		{
			input:  "CREATE DATABASE /*!32312 IF NOT EXISTS*/ `test_db` /*!40100 DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci */",
			output: "create database test_db",
		},
		{
			input:  "CREATE DATABASE `test_db` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci",
			output: "create database test_db",
		},
		{
			input:  "CREATE SCHEMA /*!32312 IF NOT EXISTS*/ `test_db` /*!40100 DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci */",
			output: "create database test_db",
		},
	}
	for _, tcase := range strictMySQLCommentDBDDL {
		tree, err := ParseStrictDDL(tcase.input)
		if err != nil {
			t.Errorf("input: %s, err: %v", tcase.input, err)
			continue
		}
		if got := String(tree); got != tcase.output {
			t.Errorf("ParseStrictDDL(%s):\n%s, want\n%s", tcase.input, got, tcase.output)
		}
	}

	validAlterTableOptionsSQL := []string{
		"alter table tbl_a add index idx_ab (col_a, col_b), algorithm=inplace, lock=none",
		"alter table tbl_a add index idx_ab (col_a, col_b), algorithm inplace, lock none",
		"alter table tbl_a add constraint uq_ab unique key idx_ab (col_a, col_b), lock=shared",
		"alter table tbl_a drop index idx_ab, lock=shared",
		"alter table tbl_a rename index idx_old to idx_new, algorithm=instant",
	}
	for _, sql := range validAlterTableOptionsSQL {
		tree, err := ParseStrictDDL(sql)
		if tree == nil || err != nil {
			t.Errorf("ParseStrictDDL unexpectedly rejected input %s: %v", sql, err)
		}
	}

	// Regression coverage: existing ALTER TABLE forms should still parse without trailing options.
	validAlterTableRegressionSQL := []string{
		"alter table tbl_a add index idx_ab (col_a, col_b)",
		"alter table tbl_a add (col_a int)",
		"alter table tbl_a drop foreign key fk_ab",
	}
	for _, sql := range validAlterTableRegressionSQL {
		tree, err := ParseStrictDDL(sql)
		if tree == nil || err != nil {
			t.Errorf("ParseStrictDDL unexpectedly rejected input %s: %v", sql, err)
		}
	}

	validAlterTableMultiSpecSQL := []string{
		"alter table tbl_b drop index idx_c1, drop primary key, add primary key (c1), add unique key uq_c2_c3 (c2, c3)",
		"alter table tbl_c add index idx_c1 (c1), drop index idx_c1",
		"alter table tbl_c2 drop column c2",
		"alter table tbl_d add (c4 int), drop foreign key fk_c4",
		"alter table tbl_h add index idx_c6 (c6), lock shared, algorithm inplace",
		"alter table tbl_i drop key idx_c7, modify c7 varchar(128) not null, add column c8 binary(16) not null, add unique (c8)",
		"alter table tbl_m drop key idx_c9, modify column c9 varchar(64) not null, add unique (c9)",
		"alter table tbl_n drop key idx_c10, add constraint primary key (c10), add constraint uq_c11 unique key (c11)",
		"alter table tbl_j add column c1 int not null, lock=shared",
		"alter table tbl_pos3 add column col_a int after col_b, lock=shared",
	}
	for _, sql := range validAlterTableMultiSpecSQL {
		tree, err := ParseStrictDDL(sql)
		if tree == nil || err != nil {
			t.Errorf("ParseStrictDDL unexpectedly rejected multi-spec input %s: %v", sql, err)
		}
	}

	validAlterTableColumnPositionSQL := []string{
		"alter table tbl_pos1 add column col_new varchar(64) default null after col_prev",
		"alter table tbl_pos1 add column col_new int first",
		"alter table tbl_pos2 add col_new int after col_prev",
		"alter table `tbl_pos4` add column `col_new` int after `col_prev`",
		"alter table tbl_pos4 modify col_new int after col_prev",
		"alter table tbl_pos4 modify column col_new int first",
		"alter table tbl_pos4 drop column col_new",
		"alter table tbl_pos4 drop col_new",
		"alter table tbl_pos5 drop key idx_a, modify col_new int after col_prev",
		"alter table tbl_pos6 add unique (col_a), modify column col_a varchar(16) first",
	}
	for _, sql := range validAlterTableColumnPositionSQL {
		tree, err := ParseStrictDDL(sql)
		if tree == nil || err != nil {
			t.Errorf("ParseStrictDDL unexpectedly rejected column-position input %s: %v", sql, err)
		}
	}

	validBooleanColumnDDL := []string{
		"alter table tbl_bool add col_flag boolean not null default false",
		"alter table tbl_bool add col_toggle bool default true",
		"alter table tbl_bool add col_enabled boolean default (true)",
		"alter table tbl_bool add col_disabled bool default (false)",
		"create table tbl_bool_create (col_flag boolean not null default false, col_toggle bool default true)",
	}
	for _, sql := range validBooleanColumnDDL {
		tree, err := ParseStrictDDL(sql)
		if tree == nil || err != nil {
			t.Errorf("ParseStrictDDL unexpectedly rejected boolean column DDL %s: %v", sql, err)
		}
	}

	sql := "create table t garbage"
	tree, err := Parse(sql)
	if err != nil {
		t.Errorf("input: %s, err: %v", sql, err)
	}

	tree, err = ParseStrictDDL(sql)
	if tree != nil || err == nil {
		t.Errorf("ParseStrictDDL unexpectedly accepted input %s", sql)
	}

	invalidCreateTableGeneratedSQL := []string{
		"create table t (total int generated as (1))",
		"create table t (total int generated always as 1)",
		"create table t (total int generated always (1))",
		"create table t (total int generated always as (1) nonsense)",
	}
	for _, sql := range invalidCreateTableGeneratedSQL {
		tree, err := ParseStrictDDL(sql)
		if tree != nil || err == nil {
			t.Errorf("ParseStrictDDL unexpectedly accepted input %s", sql)
		}
	}

	invalidAlterTableOptionsSQL := []string{
		// ALGORITHM option requires a value.
		"alter table tbl_a add index idx_ab (col_a, col_b), algorithm=",
		// LOCK option requires a value.
		"alter table tbl_a add index idx_ab (col_a, col_b), lock=",
		// Trailing comma after alter options is invalid.
		"alter table tbl_a add index idx_ab (col_a, col_b), algorithm=inplace,",
		// Trailing comma after the final alter option is invalid.
		"alter table tbl_a add index idx_ab (col_a, col_b), algorithm=inplace, lock=none,",
		// Unknown alter option key is invalid.
		"alter table tbl_a add index idx_ab (col_a, col_b), optimizer=instant",
		// Unknown alter option key is invalid even without '='.
		"alter table tbl_a add index idx_ab (col_a, col_b), optimizer instant",
		// LOCK only accepts DEFAULT, NONE, SHARED, or EXCLUSIVE.
		"alter table tbl_a add index idx_ab (col_a, col_b), lock=fast",
		// ALGORITHM only accepts DEFAULT, INSTANT, INPLACE, or COPY.
		"alter table tbl_a add index idx_ab (col_a, col_b), algorithm=online",
	}
	for _, sql := range invalidAlterTableOptionsSQL {
		tree, err := ParseStrictDDL(sql)
		if tree != nil || err == nil {
			t.Errorf("ParseStrictDDL unexpectedly accepted input %s", sql)
		}
	}

	invalidAlterTableMultiSpecSQL := []string{
		// Trailing comma after final alter-table item is invalid.
		"alter table tbl_e drop index idx_c1,",
		// Empty alter-table item between commas is invalid.
		"alter table tbl_f drop index idx_c1,, add index idx_c2 (c2)",
		// Unknown token after comma does not form a valid alter-table item.
		"alter table tbl_g add index idx_c1 (c1), ???",
		// MODIFY requires a complete column definition.
		"alter table tbl_k drop key idx_c1, modify c2",
		// ADD COLUMN requires a complete column definition.
		"alter table tbl_l add column",
		// DROP COLUMN requires a column identifier.
		"alter table tbl_l2 drop column",
	}
	for _, sql := range invalidAlterTableMultiSpecSQL {
		tree, err := ParseStrictDDL(sql)
		if tree != nil || err == nil {
			t.Errorf("ParseStrictDDL unexpectedly accepted malformed multi-spec input %s", sql)
		}
	}

	invalidAlterTableIncompleteSQL := []string{
		"alter table tbl_incomplete add constraint",
		"alter table tbl_incomplete add foreign key",
		"alter table tbl_incomplete add primary",
		"alter table tbl_incomplete add id",
		"alter table tbl_incomplete modify col_a",
		"alter table tbl_incomplete modify column col_a",
		"alter table tbl_incomplete drop foreign key",
		"alter table tbl_incomplete drop constraint",
		"alter table tbl_incomplete rename key",
		"alter table tbl_incomplete drop",
	}
	for _, sql := range invalidAlterTableIncompleteSQL {
		tree, err := ParseStrictDDL(sql)
		if tree != nil || err == nil {
			t.Errorf("ParseStrictDDL unexpectedly accepted incomplete alter-table input %s", sql)
		}
	}

	invalidAlterTableColumnPositionSQL := []string{
		// AFTER requires a target column identifier.
		"alter table tbl_pos_bad1 add column col_new int after",
		// FIRST must not be followed by another identifier.
		"alter table tbl_pos_bad2 add column col_new int first col_prev",
		// Only FIRST and AFTER are valid column position clauses.
		"alter table tbl_pos_bad3 add column col_new int before col_prev",
		// MODIFY also requires a target column identifier after AFTER.
		"alter table tbl_pos_bad4 drop key idx_a, modify col_new int after",
		// FIRST must terminate the MODIFY position clause.
		"alter table tbl_pos_bad5 drop key idx_a, modify column col_new int first col_prev",
		// Single-spec MODIFY also requires a target column identifier after AFTER.
		"alter table tbl_pos_bad6 modify col_new int after",
	}
	for _, sql := range invalidAlterTableColumnPositionSQL {
		tree, err := ParseStrictDDL(sql)
		if tree != nil || err == nil {
			t.Errorf("ParseStrictDDL unexpectedly accepted malformed column-position input %s", sql)
		}
	}

	invalidForeignKeySQL := []string{
		// Missing referenced column list after REFERENCES table_name.
		"create table t (id int, parent_id int, foreign key (parent_id) references parent)",
		// Referenced columns must be enclosed in parentheses.
		"create table t (id int, parent_id int, foreign key (parent_id) references parent id)",
		// ALTER TABLE FK add also requires referenced columns.
		"alter table t add foreign key (parent_id) references parent",
		// Local FK columns must appear as a parenthesized list after FOREIGN KEY.
		"alter table t add foreign key parent_id references parent (id)",
		// Inline REFERENCES also requires a referenced column list.
		"create table t (parent_id int references parent)",
		// Inline referenced columns must be enclosed in parentheses.
		"create table t (parent_id int references parent id)",
		// Inline REFERENCES in ALTER TABLE ... ADD (...) also requires a referenced column list.
		"alter table t add (parent_id int references parent)",
		// Inline referenced columns in ALTER TABLE ... ADD (...) must be enclosed in parentheses.
		"alter table t add (parent_id int references parent id)",
		// CONSTRAINT followed by an identifier still requires a valid foreign key body.
		"create table t (id int, parent_id int, constraint fk_parent key (parent_id) references parent (id))",
		// ALTER TABLE ... ADD CONSTRAINT also requires a valid foreign key body.
		"alter table t add constraint fk_parent key (parent_id) references parent (id)",
	}
	for _, sql := range invalidForeignKeySQL {
		tree, err := ParseStrictDDL(sql)
		if tree != nil || err == nil {
			t.Errorf("ParseStrictDDL unexpectedly accepted input %s", sql)
		}
	}

	invalidAlterAddConstraintUniqueSQL := []string{
		// Missing index type/body after constraint name.
		"alter table t add constraint stable_id_uniq unique",
		// Missing index column list.
		"alter table t add constraint stable_id_uniq unique key",
	}
	for _, sql := range invalidAlterAddConstraintUniqueSQL {
		tree, err := ParseStrictDDL(sql)
		if tree != nil || err == nil {
			t.Errorf("ParseStrictDDL unexpectedly accepted input %s", sql)
		}
	}

	invalidCreateTableConstraintUniqueSQL := []string{
		// Missing index column list in CREATE TABLE named unique constraint.
		"create table t (id int, constraint uq_t unique key)",
		// Missing index type/body in CREATE TABLE named unique constraint.
		"create table t (id int, constraint uq_t unique)",
	}
	for _, sql := range invalidCreateTableConstraintUniqueSQL {
		tree, err := ParseStrictDDL(sql)
		if tree != nil || err == nil {
			t.Errorf("ParseStrictDDL unexpectedly accepted input %s", sql)
		}
	}

	invalidCreateTablePartitionSQL := []string{
		// Missing partition definition list after PARTITION BY RANGE (...).
		"create table t (id int) partition by range (id)",
		// RANGE expression must be parenthesized.
		"create table t (id int) partition by range id (partition p0 values less than (10))",
		// VALUES LESS THAN limit must be parenthesized for non-MAXVALUE expressions.
		"create table t (id int) partition by range (id) (partition p0 values less than 10)",
		// ENGINE option requires a value.
		"create table t (id int) partition by range (id) (partition p0 values less than (10) engine =)",
		// STORAGE ENGINE option requires the ENGINE keyword.
		"create table t (id int) partition by range (id) (partition p0 values less than (10) storage = InnoDB)",
	}
	for _, sql := range invalidCreateTablePartitionSQL {
		tree, err := ParseStrictDDL(sql)
		if tree != nil || err == nil {
			t.Errorf("ParseStrictDDL unexpectedly accepted input %s", sql)
		}
	}

	invalidParenthesizedDefaultExprSQL := []string{
		// Parenthesized defaults are supported for literals, not arbitrary expressions.
		"create table t (id int default (1 + 1))",
		"create table t (config json default (json_object()))",
		"alter table t add (id int default (1 + 1))",
		"alter table t add (config json default (json_object()))",
	}
	for _, sql := range invalidParenthesizedDefaultExprSQL {
		tree, err := ParseStrictDDL(sql)
		if tree != nil || err == nil {
			t.Errorf("ParseStrictDDL unexpectedly accepted input %s", sql)
		}
	}

	invalidBooleanDefaultSQL := []string{
		// DEFAULT accepts TRUE/FALSE literals only, not boolean expressions.
		"create table t (b boolean default (true and false))",
		// NOT TRUE is valid in IS predicates, not as a DEFAULT literal.
		"create table t (b boolean default (not true))",
		// DEFAULT must have exactly one literal value.
		"create table t (b boolean default false true)",
		// ADD COLUMN follows the same DEFAULT literal restrictions as CREATE TABLE.
		"alter table t add (b bool default (true or false))",
	}
	for _, sql := range invalidBooleanDefaultSQL {
		tree, err := ParseStrictDDL(sql)
		if tree != nil || err == nil {
			t.Errorf("ParseStrictDDL unexpectedly accepted input %s", sql)
		}
	}

	testCases := []struct {
		input  string
		output string
	}{{
		// test key_block_size
		input: "create table t (\n" +
			"	id int auto_increment,\n" +
			"	username varchar,\n" +
			"	unique key by_username (username) key_block_size 8,\n" +
			"	unique key by_username2 (username) key_block_size=8,\n" +
			"	unique by_username3 (username) key_block_size = 4\n" +
			")",
		output: "create table t (\n" +
			"	id int auto_increment,\n" +
			"	username varchar,\n" +
			"	unique key by_username (username) key_block_size 8,\n" +
			"	unique key by_username2 (username) key_block_size 8,\n" +
			"	unique by_username3 (username) key_block_size 4\n" +
			")",
	}, {
		input:  "CREATE TEMPORARY TABLE IF NOT EXISTS t LIKE src",
		output: "create temporary table if not exists t like src",
	}, {
		input: "create table t (\n" +
			"	id bigint(20) unsigned not null auto_increment,\n" +
			"	j json not null default ('{}'),\n" +
			"	i tinyint(1) not null default (1),\n" +
			"	ts timestamp(3) not null default (current_timestamp)\n" +
			")",
		output: "create table t (\n" +
			"\tid bigint(20) unsigned not null auto_increment,\n" +
			"\tj json not null default '{}',\n" +
			"\ti tinyint(1) not null default 1,\n" +
			"\tts timestamp(3) not null default current_timestamp\n" +
			")",
	}, {
		input: "alter table t add (\n" +
			"	j json not null default ('{}'),\n" +
			"	i tinyint(1) not null default (1),\n" +
			"	ts timestamp(3) not null default (current_timestamp)\n" +
			")",
		output: "alter table t add (\n" +
			"\tj json not null default '{}',\n" +
			"\ti tinyint(1) not null default 1,\n" +
			"\tts timestamp(3) not null default current_timestamp\n" +
			")",
	}, {
		input: "create table t (\n" +
			"	b0 boolean default (true),\n" +
			"	b1 bool default false\n" +
			")",
		output: "create table t (\n" +
			"\tb0 boolean default true,\n" +
			"\tb1 bool default false\n" +
			")",
	}, {
		input: "create table t (\n" +
			"	id int not null primary key auto_increment,\n" +
			"	alt_id int not null auto_increment primary key\n" +
			")",
		output: "create table t (\n" +
			"\tid int not null auto_increment primary key,\n" +
			"\talt_id int not null auto_increment primary key\n" +
			")",
	}, {
		input: "create table t (\n" +
			"	status int,\n" +
			"	variables int,\n" +
			"	offset int,\n" +
			"	view int,\n" +
			"	date int,\n" +
			"	unique key status (status),\n" +
			"	unique variables (variables),\n" +
			"	index offset (offset),\n" +
			"	key view (view)\n" +
			")",
		output: "create table t (\n" +
			"\t`status` int,\n" +
			"\t`variables` int,\n" +
			"\t`offset` int,\n" +
			"\t`view` int,\n" +
			"\t`date` int,\n" +
			"\tunique key `status` (`status`),\n" +
			"\tunique `variables` (`variables`),\n" +
			"\tindex `offset` (`offset`),\n" +
			"\tkey `view` (`view`)\n" +
			")",
	}, {
		input:  "alter table t add constraint fk_parent foreign key (parent_id) references parent (id)",
		output: "alter table t add constraint fk_parent foreign key (parent_id) references parent (id)",
	}, {
		input:  "alter table stable_ids add constraint stable_id_uniq unique key (site_id, document_id, locale, feature, source)",
		output: "alter table stable_ids add unique key stable_id_uniq (site_id, document_id, locale, feature, source)",
	}, {
		input: "create table tbl_a (\n" +
			"	col_id int,\n" +
			"	col_a int,\n" +
			"	col_b int,\n" +
			"	constraint uq_generic unique index (col_a, col_b)\n" +
			")",
		output: "create table tbl_a (\n" +
			"\tcol_id int,\n" +
			"\tcol_a int,\n" +
			"\tcol_b int,\n" +
			"\tunique index uq_generic (col_a, col_b)\n" +
			")",
	}, {
		input: "create table tbl_b (\n" +
			"	col_id int,\n" +
			"	col_c int,\n" +
			"	constraint uq_generic_key unique key (col_c)\n" +
			")",
		output: "create table tbl_b (\n" +
			"\tcol_id int,\n" +
			"\tcol_c int,\n" +
			"\tunique key uq_generic_key (col_c)\n" +
			")",
	}, {
		input:  "alter table t add constraint foreign key (parent_id) references parent (id)",
		output: "alter table t add foreign key (parent_id) references parent (id)",
	}, {
		input: "create table t (\n" +
			"	id int,\n" +
			"	parent_id int,\n" +
			"	constraint foreign key (parent_id) references parent (id)\n" +
			")",
		output: "create table t (\n" +
			"\tid int,\n" +
			"\tparent_id int,\n" +
			"\tforeign key (parent_id) references parent (id)\n" +
			")",
	}, {
		input: "create table t (\n" +
			"	id int,\n" +
			"	parent_id int,\n" +
			"	secondary_parent_id int,\n" +
			"	constraint foreign key (parent_id) references parent (id),\n" +
			"	constraint fk_secondary_parent foreign key (secondary_parent_id) references parent (id)\n" +
			")",
		output: "create table t (\n" +
			"\tid int,\n" +
			"\tparent_id int,\n" +
			"\tsecondary_parent_id int,\n" +
			"\tforeign key (parent_id) references parent (id),\n" +
			"\tconstraint fk_secondary_parent foreign key (secondary_parent_id) references parent (id)\n" +
			")",
	}, {
		input:  "alter table t add foreign key (parent_id) references parent (id) on delete set null on update cascade",
		output: "alter table t add foreign key (parent_id) references parent (id) on delete set null on update cascade",
	}, {
		input:  "create table t (\n\tparent_id int references parent (id)\n)",
		output: "create table t (\n\tparent_id int references parent (id)\n)",
	}, {
		input:  "create table t (\n\tparent_id int not null references parent (id) on delete set null on update cascade\n)",
		output: "create table t (\n\tparent_id int not null references parent (id) on delete set null on update cascade\n)",
	}, {
		input:  "alter table t add (\n\tparent_id int references parent (id)\n)",
		output: "alter table t add (\n\tparent_id int references parent (id)\n)",
	}, {
		input:  "alter table t add (\n\tparent_id int not null references parent (id) on delete set null on update cascade\n)",
		output: "alter table t add (\n\tparent_id int not null references parent (id) on delete set null on update cascade\n)",
	}, {
		input:  "alter table t drop foreign key fk_parent",
		output: "alter table t drop foreign key fk_parent",
	}, {
		input: "create table t (\n" +
			"	id bigint(20) unsigned auto_increment not null,\n" +
			"	id2 bigint(20) unsigned not null auto_increment\n" +
			")",
		output: "create table t (\n" +
			"\tid bigint(20) unsigned not null auto_increment,\n" +
			"\tid2 bigint(20) unsigned not null auto_increment\n" +
			")",
	}, {
		input: "create table t (\n" +
			"	id bigint unsigned not null auto_increment,\n" +
			"	primary key (id)\n" +
			") partition by range (id) (\n" +
			"	partition p0 values less than (100),\n" +
			"	partition p1 values less than maxvalue\n" +
			")",
		output: "create table t (\n" +
			"\tid bigint unsigned not null auto_increment,\n" +
			"\tprimary key (id)\n" +
			") partition by range (id) (partition p0 values less than (100), partition p1 values less than (maxvalue))",
	}, {
		input: "create table t (\n" +
			"	id bigint unsigned not null auto_increment,\n" +
			"	primary key (id)\n" +
			") partition by range (id) (\n" +
			"	partition p0 values less than (100) engine = InnoDB,\n" +
			"	partition p1 values less than maxvalue engine InnoDB\n" +
			")",
		output: "create table t (\n" +
			"\tid bigint unsigned not null auto_increment,\n" +
			"\tprimary key (id)\n" +
			") partition by range (id) (partition p0 values less than (100) engine InnoDB, partition p1 values less than (maxvalue) engine InnoDB)",
	}, {
		input: "create table t (\n" +
			"	id bigint unsigned not null auto_increment,\n" +
			"	primary key (id)\n" +
			") partition by range (id) (\n" +
			"	partition p0 values less than (100) storage engine InnoDB,\n" +
			"	partition p1 values less than maxvalue storage engine=InnoDB\n" +
			")",
		output: "create table t (\n" +
			"\tid bigint unsigned not null auto_increment,\n" +
			"\tprimary key (id)\n" +
			") partition by range (id) (partition p0 values less than (100) engine InnoDB, partition p1 values less than (maxvalue) engine InnoDB)",
	}, {
		input: "create table update_files (\n" +
			"	update_id bigint(20) not null,\n" +
			"	path varchar(1024) collate utf8mb4_unicode_ci not null,\n" +
			"	update_type enum('ADD','DELETE') character set ascii collate ascii_bin not null,\n" +
			"	redirect_location varchar(1024) collate utf8mb4_unicode_ci default null,\n" +
			"	already_exists_and_equal tinyint(1) default null,\n" +
			"	key update_id (update_id)\n" +
			") engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci /*!50100 partition by range (update_id)\n" +
			"(partition id20200000 values less than (20200000) engine = InnoDB,\n" +
			" partition latest values less than maxvalue engine = InnoDB) */",
		output: "create table update_files (\n" +
			"\tupdate_id bigint(20) not null,\n" +
			"\t`path` varchar(1024) collate utf8mb4_unicode_ci not null,\n" +
			"\tupdate_type enum('ADD', 'DELETE') character set ascii collate ascii_bin not null,\n" +
			"\tredirect_location varchar(1024) collate utf8mb4_unicode_ci default null,\n" +
			"\talready_exists_and_equal tinyint(1) default null,\n" +
			"\tkey update_id (update_id)\n" +
			") engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci partition by range (update_id) (partition id20200000 values less than (20200000) engine InnoDB, partition latest values less than (maxvalue) engine InnoDB)",
	},
	}
	for _, tcase := range testCases {
		tree, err := ParseStrictDDL(tcase.input)
		if err != nil {
			t.Errorf("input: %s, err: %v", tcase.input, err)
			continue
		}
		if got, want := String(tree.(*DDL)), tcase.output; got != want {
			t.Errorf("Parse(%s):\n%s, want\n%s", tcase.input, got, want)
		}
	}

	keywordIndexDDLs := []string{
		"create index status using btree on t (id)",
		"drop index status on t",
	}
	for _, sql := range keywordIndexDDLs {
		if _, err := ParseStrictDDL(sql); err != nil {
			t.Errorf("input: %s, err: %v", sql, err)
		}
	}
}

func TestCreateTableEscaped(t *testing.T) {
	testCases := []struct {
		input  string
		output string
	}{{
		input: "create table `a`(`id` int, primary key(`id`))",
		output: "create table a (\n" +
			"\tid int,\n" +
			"\tprimary key (id)\n" +
			")",
	}, {
		input: "create table `insert`(`update` int, primary key(`delete`))",
		output: "create table `insert` (\n" +
			"\t`update` int,\n" +
			"\tprimary key (`delete`)\n" +
			")",
	}}
	for _, tcase := range testCases {
		tree, err := ParseStrictDDL(tcase.input)
		if err != nil {
			t.Errorf("input: %s, err: %v", tcase.input, err)
			continue
		}
		if got, want := String(tree.(*DDL)), tcase.output; got != want {
			t.Errorf("Parse(%s):\n%s, want\n%s", tcase.input, got, want)
		}
	}
}

var (
	invalidSQL = []struct {
		input        string
		output       string
		excludeMulti bool // Don't use in the ParseNext multi-statement parsing tests.
	}{{
		input:  "select $ from t",
		output: "syntax error at position 9 near '$'",
	}, {
		input:  "select : from t",
		output: "syntax error at position 9 near ':'",
	}, {
		input:  "select 0xH from t",
		output: "syntax error at position 10 near '0x'",
	}, {
		input:  "select x'78 from t",
		output: "syntax error at position 12 near '78'",
	}, {
		input:  "select x'777' from t",
		output: "syntax error at position 14 near '777'",
	}, {
		input:  "select * from t where :1 = 2",
		output: "syntax error at position 24 near ':'",
	}, {
		input:  "select * from t where :. = 2",
		output: "syntax error at position 24 near ':'",
	}, {
		input:  "select * from t where ::1 = 2",
		output: "syntax error at position 24 near ':'",
	}, {
		input:  "select * from t where ::. = 2",
		output: "syntax error at position 24 near ':'",
	}, {
		input:  "update a set c = values(1)",
		output: "syntax error at position 26 near '1'",
	}, {
		input: "select(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F" +
			"(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(" +
			"F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F" +
			"(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(" +
			"F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F" +
			"(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(" +
			"F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F" +
			"(F(F(F(F(F(F(F(F(F(F(F(F(",
		output: "max nesting level reached at position 406",
	}, {
		input: "select(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F" +
			"(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(" +
			"F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F" +
			"(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(" +
			"F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F" +
			"(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(" +
			"F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F" +
			"(F(F(F(F(F(F(F(F(F(F(F(",
		output: "syntax error at position 404",
	}, {
		// This construct is considered invalid due to a grammar conflict.
		input:  "insert into a select * from b join c on duplicate key update d=e",
		output: "syntax error at position 54 near 'key'",
	}, {
		input:  "select * from a left join b",
		output: "syntax error at position 28",
	}, {
		input:  "select * from a natural join b on c = d",
		output: "syntax error at position 34 near 'on'",
	}, {
		input:  "select * from a natural join b using (c)",
		output: "syntax error at position 37 near 'using'",
	}, {
		input:  "select next 1+1 values from a",
		output: "syntax error at position 14 near '1'",
	}, {
		input:  "insert into a values (select * from b)",
		output: "syntax error at position 29 near 'select'",
	}, {
		input:  "select database",
		output: "syntax error at position 16",
	}, {
		input:  "select mod from t",
		output: "syntax error at position 16 near 'from'",
	}, {
		input:  "select 1 from t where div 5",
		output: "syntax error at position 26 near 'div'",
	}, {
		input:  "select 1 from t where binary",
		output: "syntax error at position 29",
	}, {
		input:  "select match(a1, a2) against ('foo' in boolean mode with query expansion) from t",
		output: "syntax error at position 57 near 'with'",
	}, {
		input:  "select /* reserved keyword as unqualified column */ * from t where key = 'test'",
		output: "syntax error at position 71 near 'key'",
	}, {
		input:  "select /* vitess-reserved keyword as unqualified column */ * from t where escape = 'test'",
		output: "syntax error at position 81 near 'escape'",
	}, {
		input:  "(select /* parenthesized select */ * from t)",
		output: "syntax error at position 45",
	}, {
		input:  "select * from t where id = ((select a from t1 union select b from t2) order by a limit 1)",
		output: "syntax error at position 76 near 'order'",
	}, {
		input:  "select /* straight_join using */ 1 from t1 straight_join t2 using (a)",
		output: "syntax error at position 66 near 'using'",
	}, {
		input:        "select 'aa",
		output:       "syntax error at position 11 near 'aa'",
		excludeMulti: true,
	}, {
		input:        "select 'aa\\",
		output:       "syntax error at position 12 near 'aa'",
		excludeMulti: true,
	}, {
		input:        "select /* aa",
		output:       "syntax error at position 13 near '/* aa'",
		excludeMulti: true,
	}}
)

func TestErrors(t *testing.T) {
	for _, tcase := range invalidSQL {
		_, err := Parse(tcase.input)
		if err == nil || err.Error() != tcase.output {
			t.Errorf("%s: %v, want %s", tcase.input, err, tcase.output)
		}
	}
}

// Benchmark run on 6/23/17, prior to improvements:
// BenchmarkParse1-4         100000             16334 ns/op
// BenchmarkParse2-4          30000             44121 ns/op

func BenchmarkParse1(b *testing.B) {
	sql := "select 'abcd', 20, 30.0, eid from a where 1=eid and name='3'"
	for i := 0; i < b.N; i++ {
		ast, err := Parse(sql)
		if err != nil {
			b.Fatal(err)
		}
		_ = String(ast)
	}
}

func BenchmarkParse2(b *testing.B) {
	sql := "select aaaa, bbb, ccc, ddd, eeee, ffff, gggg, hhhh, iiii from tttt, ttt1, ttt3 where aaaa = bbbb and bbbb = cccc and dddd+1 = eeee group by fff, gggg having hhhh = iiii and iiii = jjjj order by kkkk, llll limit 3, 4"
	for i := 0; i < b.N; i++ {
		ast, err := Parse(sql)
		if err != nil {
			b.Fatal(err)
		}
		_ = String(ast)
	}
}

var benchQuery string

func init() {
	// benchQuerySize is the approximate size of the query.
	benchQuerySize := 1000000

	// Size of value is 1/10 size of query. Then we add
	// 10 such values to the where clause.
	var baseval bytes.Buffer
	for i := 0; i < benchQuerySize/100; i++ {
		// Add an escape character: This will force the upcoming
		// tokenizer improvement to still create a copy of the string.
		// Then we can see if avoiding the copy will be worth it.
		baseval.WriteString("\\'123456789")
	}

	var buf bytes.Buffer
	buf.WriteString("select a from t1 where v = 1")
	for i := 0; i < 10; i++ {
		fmt.Fprintf(&buf, " and v%d = \"%d%s\"", i, i, baseval.String())
	}
	benchQuery = buf.String()
}

func BenchmarkParse3(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := Parse(benchQuery); err != nil {
			b.Fatal(err)
		}
	}
}
