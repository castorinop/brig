package fixtures

import (
	. "gx/ipfs/QmPAdjGx1huCjnrR26qy9QUUNSqA6EStyZ68RrwbtCTDML/refmt/tok"
)

var sequences_Composite = []Sequence{
	{"array nested in map as non-first and final entry",
		[]Token{
			{Type: TMapOpen, Length: 2},
			TokStr("k1"),
			TokStr("v1"),
			TokStr("ke"),
			{Type: TArrOpen, Length: 3},
			TokStr("oh"),
			TokStr("whee"),
			TokStr("wow"),
			{Type: TArrClose},
			{Type: TMapClose},
		},
	},
	{"array nested in map as first and non-final entry",
		[]Token{
			{Type: TMapOpen, Length: 2},
			TokStr("ke"),
			{Type: TArrOpen, Length: 3},
			TokStr("oh"),
			TokStr("whee"),
			TokStr("wow"),
			{Type: TArrClose},
			TokStr("k1"),
			TokStr("v1"),
			{Type: TMapClose},
		},
	},
	{"maps nested in array",
		[]Token{
			{Type: TArrOpen, Length: 3},
			{Type: TMapOpen, Length: 1},
			TokStr("k"),
			TokStr("v"),
			{Type: TMapClose},
			TokStr("whee"),
			{Type: TMapOpen, Length: 1},
			TokStr("k1"),
			TokStr("v1"),
			{Type: TMapClose},
			{Type: TArrClose},
		},
	},
	{"arrays in arrays in arrays",
		[]Token{
			{Type: TArrOpen, Length: 1},
			{Type: TArrOpen, Length: 1},
			{Type: TArrOpen, Length: 0},
			{Type: TArrClose},
			{Type: TArrClose},
			{Type: TArrClose},
		},
	},
	{"maps nested in maps",
		[]Token{
			{Type: TMapOpen, Length: 1},
			TokStr("k"),
			{Type: TMapOpen, Length: 1},
			TokStr("k2"),
			TokStr("v2"),
			{Type: TMapClose},
			{Type: TMapClose},
		},
	},
	{"empty map nested in map", // you wouldn't think this be interesting, but obj sometimes has fun here.
		[]Token{
			{Type: TMapOpen, Length: 1},
			TokStr("k"),
			{Type: TMapOpen, Length: 0},
			{Type: TMapClose},
			{Type: TMapClose},
		},
	},
	{"nil nested in map",
		[]Token{
			{Type: TMapOpen, Length: 1},
			TokStr("k"),
			{Type: TNull},
			{Type: TMapClose},
		},
	},
	{"jumbles nested in map",
		[]Token{
			{Type: TMapOpen, Length: 4},
			TokStr("s"),
			{Type: TString, Str: "foo"},
			TokStr("m"),
			{Type: TMapOpen, Length: 0},
			{Type: TMapClose},
			TokStr("i"),
			{Type: TInt, Int: 42},
			TokStr("k"),
			{Type: TNull},
			{Type: TMapClose},
		},
	},
	{"maps nested in maps with mixed nulls",
		[]Token{
			{Type: TMapOpen, Length: 2},
			TokStr("k"),
			{Type: TMapOpen, Length: 1},
			TokStr("k2"),
			TokStr("v2"),
			{Type: TMapClose},
			TokStr("k2"),
			{Type: TNull},
			{Type: TMapClose},
		},
	},
	{"map[str][]map[str]int",
		// this one is primarily for the objmapper tests
		[]Token{
			{Type: TMapOpen, Length: 1},
			TokStr("k"),
			{Type: TArrOpen, Length: 2},
			{Type: TMapOpen, Length: 1},
			TokStr("k2"),
			TokInt(1),
			{Type: TMapClose},
			{Type: TMapOpen, Length: 1},
			TokStr("k2"),
			TokInt(2),
			{Type: TMapClose},
			{Type: TArrClose},
			{Type: TMapClose},
		},
	},
	{"map[str]map[str]map[str]str",
		// this is primarily for the objmapper tests (map-struct-map case).
		[]Token{
			{Type: TMapOpen, Length: 2},
			TokStr("k1"), {Type: TMapOpen, Length: 1},
			/**/ TokStr("f"), {Type: TMapOpen, Length: 1},
			/**/ /**/ TokStr("d"), TokStr("aa"),
			/**/ /**/ {Type: TMapClose},
			/**/ {Type: TMapClose},
			TokStr("k2"), {Type: TMapOpen, Length: 1},
			/**/ TokStr("f"), {Type: TMapOpen, Length: 1},
			/**/ /**/ TokStr("d"), TokStr("bb"),
			/**/ /**/ {Type: TMapClose},
			/**/ {Type: TMapClose},
			{Type: TMapClose},
		},
	},
}
