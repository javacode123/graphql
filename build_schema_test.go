package graphql_test

import (
	"reflect"
	"testing"

	"github.com/graphql-go/graphql"
)

func TestSimpleTypes(t *testing.T) {
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

func TestExcludedStandardTypes(t *testing.T) {
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

func TestDirectives(t *testing.T) {
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

func TestTypeModifiers(t *testing.T) {
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

func TestRecursiveType(t *testing.T) {
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

func TestTwoCircularTypes(t *testing.T) {
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

func TestSimpleTypeWithInterface(t *testing.T) {
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

func TestSimpleOutputEnum(t *testing.T) {
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

func TestMultiValueEnum(t *testing.T) {
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
		enumNames := []string{enumType.Values()[0].Name, enumType.Values()[1].Name}
		if !(reflect.DeepEqual(enumNames, []string{"WO", "RLD"}) ||
			reflect.DeepEqual(enumNames, []string{"RLD", "WO"})) {
			t.Fatalf("Enum values are %v, not 'WO' and 'RLD'", enumNames)
		}
	}
}

func TestSimpleUnion(t *testing.T) {
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

func TestMultipleUnion(t *testing.T) {
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

func TestCustomScalar(t *testing.T) {
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

func TestSimpleInputObject(t *testing.T) {
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

func TestInputWithEnumList(t *testing.T) {
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

func TestSimpleArgumentFieldWithDefault(t *testing.T) {
	sdl := `
	  type Query {
		  str(int: Int = 2): String
	  }
	`
	schema, err := graphql.BuildSchema(sdl)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
	queryField := schema.QueryType().Fields()["str"]
	if len(queryField.Args) != 1 {
		t.Fatalf("Query field has %v instead of 1 argrument", len(queryField.Args))
	} else {
		arg := queryField.Args[0]
		if arg.Name() != "int" || arg.DefaultValue != 2 {
			t.Fatalf("Unexpected field argument '%v' with default value '%v'", arg.Name(), arg.DefaultValue)
		}
	}
}

func TestCustomScalarArgumentWithDefault(t *testing.T) {
	sdl := `
	  scalar CustomScalar

	  type Query {
		 str(int: CustomScalar = 2): String
	  }
  `
	schema, err := graphql.BuildSchema(sdl)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}

	queryField := schema.QueryType().Fields()["str"]
	if len(queryField.Args) != 1 {
		t.Fatalf("Query field has %v instead of 1 argrument", len(queryField.Args))
	} else {
		arg := queryField.Args[0]
		if arg.Name() != "int" || arg.Type.Name() != "CustomScalar" || arg.DefaultValue != 2 {
			t.Fatalf("Unexpected field argument '%v' of type '%v' with default value '%v'", arg.Name(), arg.Type.Name(), arg.DefaultValue)
		}
	}
}

func TestSimpleTypeWithMutation(t *testing.T) {
	sdl := `
	  schema {
		  query: HelloScalars
		  mutation: Mutation
	  }

	  type HelloScalars {
		  str: String
		  int: Int
		  bool: Boolean
	  }

	  type Mutation {
		  addHelloScalars(str: String, int: Int, bool: Boolean): HelloScalars
	  }
	`

	_, err := graphql.BuildSchema(sdl)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
}

func TestUnreferencedTypeImplementingReferencedInterface(t *testing.T) {
	sdl := `
	  type Concrete implements Interface {
		  key: String
	  }

	   interface Interface {
		   key: String
	  }

	  type Query {
		  interface: Interface
	  }
  `

	_, err := graphql.BuildSchema(sdl)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
}

func TestUnreferencedInterfaceImplementingReferencedInterface(t *testing.T) {
	t.Skip("TODO")
}

func TestUnreferencedTypeImplementingReferencedUnion(t *testing.T) {
	t.Skip("TODO")
}

func TestDeprecatedDirective(t *testing.T) {
	t.Skip("TODO")
}

func TestSpecifiedByDirective(t *testing.T) {
	t.Skip("TODO")
}

// TODO: Add more tests from graphql-js

///////// Tests in graphql-js that do not pass because of graphql-go :(

func TestSimpleInterfaceHierarchy(t *testing.T) {
	t.Skip("graphql-go does not support interface 'implements'")

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

func TestEmptyEnum(t *testing.T) {
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

func TestEmptyInputObject(t *testing.T) {
	t.Skip("graphql-go does not support empty types")

	sdl := `
	  input Input

	  type Query {
		  field: String
	  }
	`
	_, err := graphql.BuildSchema(sdl)
	if err != nil {
		t.Fatalf("Unexpected error %s", err.Error())
	}
}
