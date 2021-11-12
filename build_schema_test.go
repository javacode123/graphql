package graphql_test

import (
	"testing"

	"github.com/graphql-go/graphql"
)

func TestBuildAstSchema_SimpleTypes(t *testing.T) {
	sdl := `
		type Query {
			str: String
			int: Int
			float: Float
			id: ID
			bool: Boolean
		}
	`
	schema, err := graphql.BuildSchema(sdl)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}

	var ttype graphql.Type

	ttype = schema.Type("Int")
	if scalarType, ok := ttype.(*graphql.Scalar); !ok || scalarType.Name() != "Int" {
		t.Fatal("No Int type")
	}

	ttype = schema.Type("Float")
	if scalarType, ok := ttype.(*graphql.Scalar); !ok || scalarType.Name() != "Float" {
		t.Fatal("No Float type")
	}

	ttype = schema.Type("String")
	if scalarType, ok := ttype.(*graphql.Scalar); !ok || scalarType.Name() != "String" {
		t.Fatal("No String type")
	}

	ttype = schema.Type("Boolean")
	if scalarType, ok := ttype.(*graphql.Scalar); !ok || scalarType.Name() != "Boolean" {
		t.Fatal("No Boolean type")
	}

	ttype = schema.Type("ID")
	if scalarType, ok := ttype.(*graphql.Scalar); !ok || scalarType.Name() != "ID" {
		t.Fatal("No ID type")
	}

	if schema.QueryType() == nil {
		t.Fatal("No Query type")
	}

	if len(schema.TypeMap()) != 6+len(graphql.GetIntrospectionTypes()) {
		t.Fatalf("Unexpected number of types: %v", schema.TypeMap())
	}
}

func TestBuildAstSchema_ExcludedStandardTypes(t *testing.T) {
	schema, err := graphql.BuildSchema("type Query { str: String }")
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	if schema.Type("Int") != nil {
		t.Fatal("Contains Int type")
	}
	if schema.Type("Float") != nil {
		t.Fatal("Contains Float type")
	}
	if schema.Type("ID") != nil {
		t.Fatal("Contains ID type")
	}

	// Gets Boolean from introspection types
	if schema.Type("Boolean") == nil {
		t.Fatal("Does not contain Boolean type")
	}
}

func TestBuildAstSchema_Directives(t *testing.T) {
	sdl := `
		directive @foo(arg: Int) on FIELD

		type Query {
			str: String
		}
	`

	schema, err := graphql.BuildSchema(sdl)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}

	if schema.Directive("foo") == nil {
		t.Fatal("Does not contain directive 'foo'")
	}

	// it still includes standard directives
	if schema.Directive("skip") == nil {
		t.Fatal("Does not contain directive 'skip'")
	}
	if schema.Directive("include") == nil {
		t.Fatal("Does not contain directive 'include'")
	}
	if schema.Directive("deprecated") == nil {
		t.Fatal("Does not contain directive 'deprecated'")
	}
	if schema.Directive("specifiedBy") == nil {
		t.Fatal("Does not contain directive 'specifiedBy'")
	}
	if len(schema.Directives()) != 5 {
		t.Fatalf("Unexpected number of directives: %d", len(schema.Directives()))
	}
}

func TestBuildAstSchema_TypeModifiers(t *testing.T) {
	sdl := `
		type Query {
			nonNullStr: String!
			listOfStrings: [String]
			listOfNonNullStrings: [String!]
			nonNullListOfStrings: [String]!
			nonNullListOfNonNullStrings: [String!]!
		}
	`

	schema, err := graphql.BuildSchema(sdl)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}

	fieldMap := schema.QueryType().Fields()

	if nonNullStr, ok := fieldMap["nonNullStr"].Type.(*graphql.NonNull); !ok {
		t.Fatal("Query.nonNullStr is not non-null")
	} else if scalar, ok := nonNullStr.OfType.(*graphql.Scalar); !ok {
		t.Fatal("Query.nonNullStr is not a scalar")
	} else if scalar.Name() != "String" {
		t.Fatalf("Query.nonNullStr is a %s", scalar.Name())
	}

	if listOfStrings, ok := fieldMap["listOfStrings"].Type.(*graphql.List); !ok {
		t.Fatal("Query.listOfStrings is not a list")
	} else if scalar, ok := listOfStrings.OfType.(*graphql.Scalar); !ok {
		t.Fatal("Query.listOfStrings is not a scalar")
	} else if scalar.Name() != "String" {
		t.Fatalf("Query.listOfStrings is a %ss", scalar.Name())
	}

	if listOfNonNullStrings, ok := fieldMap["listOfNonNullStrings"].Type.(*graphql.List); !ok {
		t.Fatal("Query.listOfNonNullStrings is not a list")
	} else if nonNullStr, ok := listOfNonNullStrings.OfType.(*graphql.NonNull); !ok {
		t.Fatal("Query.listOfNonNullStrings is not of non-nulls")
	} else if scalar, ok := nonNullStr.OfType.(*graphql.Scalar); !ok {
		t.Fatal("Query.listOfNonNullStrings is not of scalars")
	} else if scalar.Name() != "String" {
		t.Fatalf("Query.listOfNonNullStrings contains %ss", scalar.Name())
	}

	if nonNullListOfStrings, ok := fieldMap["nonNullListOfStrings"].Type.(*graphql.NonNull); !ok {
		t.Fatal("Query.nonNullListOfStrings is not non-null")
	} else if listOfStrings, ok := nonNullListOfStrings.OfType.(*graphql.List); !ok {
		t.Fatal("Query.nonNullListOfStrings is not a list")
	} else if scalar, ok := listOfStrings.OfType.(*graphql.Scalar); !ok {
		t.Fatal("Query.nonNullListOfStrings is not of scalars")
	} else if scalar.Name() != "String" {
		t.Fatalf("Query.nonNullListOfStrings contains %ss", scalar.Name())
	}

	if nonNullListOfNonNullStrings, ok := fieldMap["nonNullListOfNonNullStrings"].Type.(*graphql.NonNull); !ok {
		t.Fatal("Query.nonNullListOfNonNullStrings is not non-null")
	} else if listOfNonNullStrings, ok := nonNullListOfNonNullStrings.OfType.(*graphql.List); !ok {
		t.Fatal("Query.nonNullListOfNonNullStrings is not a list")
	} else if listOfStrings, ok := listOfNonNullStrings.OfType.(*graphql.NonNull); !ok {
		t.Fatal("Query.nonNullListOfNonNullStrings is not of non-null String")
	} else if scalar, ok := listOfStrings.OfType.(*graphql.Scalar); !ok {
		t.Fatal("Query.nonNullListOfNonNullStrings is not of scalars")
	} else if scalar.Name() != "String" {
		t.Fatalf("Query.nonNullListOfNonNullStrings contains %ss", scalar.Name())
	}
}

func TestBuildAstSchema_RecursiveType(t *testing.T) {
	sdl := `
		type Query {
			str: String
			recurse: Query
		}
	`
	_, err := graphql.BuildSchema(sdl)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
}

func TestBuildAstSchema_TwoCircularTypes(t *testing.T) {
	sdl := `
		type Query {
			str: String
			otherType: OtherType
		}

		type OtherType {
			str: String
			queryType: Query
		}
	`
	_, err := graphql.BuildSchema(sdl)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
}

func TestBuildAstSchema_SimpleTypeWithInterface(t *testing.T) {
	sdl := `
		type Query implements WorldInterface {
			str: String
		}

		interface WorldInterface {
			str: String
		}
	`
	_, err := graphql.BuildSchema(sdl)
	// interfaces := schema.QueryType().Interfaces()

	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
}

func TestBuildAstSchema_SimpleOutputEnum(t *testing.T) {
	sdl := `
		enum Hello {
			WORLD
		}

		type Query {
			hello: Hello
		}
	`

	schema, err := graphql.BuildSchema(sdl)

	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}

	if enumType, ok := schema.Type("Hello").(*graphql.Enum); !ok {
		t.Fatal("No enum type")
	} else if len(enumType.Values()) != 1 {
		t.Fatalf("Enum has %d values instead of 1", len(enumType.Values()))
	} else {
		enumValue := enumType.Values()[0]
		if enumValue.Name != "WORLD" {
			t.Fatalf("Enum value is '%s', not 'WORLD'", enumValue.Name)
		}
	}
}

func TestBuildAstSchema_MultiValueEnum(t *testing.T) {
	sdl := `
		enum Hello {
			WO
			RLD
		}

		type Query {
			hello: Hello
		}
	`

	schema, err := graphql.BuildSchema(sdl)

	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}

	if enumType, ok := schema.Type("Hello").(*graphql.Enum); !ok {
		t.Fatal("No enum type")
	} else if len(enumType.Values()) != 2 {
		t.Fatalf("Enum has %d values instead of 1", len(enumType.Values()))
	} else {
		firstValue := enumType.Values()[0]
		if firstValue.Name != "WO" {
			t.Fatalf("Enum value is '%s', not 'WO'", firstValue.Name)
		}
		secondValue := enumType.Values()[1]
		if secondValue.Name != "RLD" {
			t.Fatalf("Enum value is '%s', not 'RLD'", firstValue.Name)
		}
	}
}

func TestBuildAstSchema_SimpleUnion(t *testing.T) {
	sdl := `
	  union Hello = World

	  type Query {
	    hello: Hello
	  }

	  type World {
	    str: String
	  }
	`

	_, err := graphql.BuildSchema(sdl)

	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
}

func TestBuildAstSchema_MultipleUnion(t *testing.T) {
	sdl := `
	  union Hello = WorldOne | WorldTwo

	  type Query {
	    hello: Hello
	  }

	  type WorldOne {
	    str: String
	  }

	  type WorldTwo {
	    str: String
	  }
	`
	_, err := graphql.BuildSchema(sdl)

	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
}

func TestBuildAstSchema_CustomScalar(t *testing.T) {
	sdl := `
		scalar CustomScalar

		type Query {
			customScalar: CustomScalar
		}
	`
	schema, err := graphql.BuildSchema(sdl)

	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}

	if schema.Type("CustomScalar") == nil {
		t.Fatal("No CustomScalar type")
	}
}

func TestBuildAstSchema_SimpleInputObject(t *testing.T) {
	sdl := `
	  input Input {
		  int: Int
	  }

	  type Query {
		  field(in: Input): String
	  }
	`
	_, err := graphql.BuildSchema(sdl)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
}

func TestBuildAstSchema_InputWithEnumList(t *testing.T) {
	sdl := `
    type Query {
	    queryWithInput(filter: FilterInput): String
    }

	  enum Values {
		  A
		  B
		  C
	  }

	  input FilterInput {
		  values: [Values!]
	  }
	`
	_, err := graphql.BuildSchema(sdl)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
}

// TODO: Add more tests from graphql-js

///////// Tests in graphql-js that do not pass because of graphql-go :(

func TestBuildAstSchema_SimpleInterfaceHierarchy(t *testing.T) {
	t.Skip("graphql-go does not support interfaces implementing interfaces")

	sdl := `
		schema {
			query: Child
		}

		interface Child implements Parent {
			str: String
		}

		type Hello implements Parent & Child {
			str: String
		}

		interface Parent {
			str: String
		}
	`

	_, err := graphql.BuildSchema(sdl)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
}

func TestBuildAstSchema_EmptyEnum(t *testing.T) {
	t.Skip("graphql-go does not support empty types")

	sdl := `
		enum Empty

		type Query {
			str: String
		}
	`

	_, err := graphql.BuildSchema(sdl)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
}
