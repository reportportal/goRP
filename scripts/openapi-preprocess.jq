walk(
  if type == "object"
     and .additionalProperties
     and .additionalProperties.type == "object"
     and (.additionalProperties.additionalProperties | not)
  then .additionalProperties = true
  # Array schemas without items default to string — Go codegen requires items to be present
  elif type == "object"
     and .type == "array"
     and (.items | not)
  then .items = {"type": "string"}
  else .
  end
)
# Duplicate path with {projectName} but empty parameters — remove to avoid validation error
| del(.paths["/v1/project/{projectName}/preference/"])

# The upstream spec tags some operations with multiple tags, causing the Go generator to
# emit the same request struct in several files (e.g. api_test_case.go AND
# api_batch_operations.go), which fails compilation with "redeclared in this block".
# Keep only the first tag so every operation lands in exactly one generated file.
| .paths |= with_entries(
    .value |= with_entries(
      if (.value | type == "object") and (.value.tags | type == "array") and (.value.tags | length > 1) then
        .value.tags = [.value.tags[0]]
      else .
      end
    )
  )

# Fix: the spec declares ErrorRS.errorCode as a string enum of ErrorType names ("LAUNCH_NOT_FOUND"),
# but the server writes the numeric code: ErrorRS.java annotates the field with
# @JsonSerialize(using = ErrorTypeSerializer.class), which calls writeNumber(error.getCode()),
# so the wire format is {"errorCode":4041,"message":"..."}. With the string type the generated
# client fails to decode every such error response ("cannot unmarshal number into ... errorCode
# of type string") and the server's message is lost. Declare it the same way as the spec's own
# `Error` schema does (integer/int64). Fails loudly if upstream renames the schema or the property.
| "com.epam.reportportal.base.infrastructure.rules.exception.ErrorRS" as $errorRS
| if .components.schemas[$errorRS].properties.errorCode then
    .components.schemas[$errorRS].properties.errorCode = {
        type: "integer",
        format: "int64",
        description: "Numeric ReportPortal error code (ErrorType code), e.g. 4041 for LAUNCH_NOT_FOUND"
    }
  else
    error("openapi-preprocess: schema \($errorRS) has no errorCode property; update the errorCode override")
  end

# Fix: the Go openapi-generator does not call the parent's ToMap() from a child's
# ToMap() when allOf inheritance is used, so discriminator fields (e.g.
# manualScenarioType) are silently dropped from MarshalJSON output.
# Flatten allOf[ref + inline-properties] schemas into a single properties block
# so every field ends up in one ToMap() and the serialized JSON is complete.
| .components.schemas as $schemas
| .components.schemas |= (
    to_entries | map(
        if (
            .value | type == "object"
            and has("allOf")
            and (.allOf | length == 2)
            and (.allOf[0] | has("$ref"))
            and (.allOf[1] | has("properties"))
        ) then
            .value.allOf[0]["$ref"] as $ref |
            ($ref | ltrimstr("#/components/schemas/")) as $parentKey |
            ($schemas[$parentKey].properties // {}) as $parentProps |
            ($schemas[$parentKey].required // []) as $parentRequired |
            .value |= {
                required: ((.required // []) + $parentRequired + (.allOf[1].required // []) | unique),
                type: "object",
                properties: ($parentProps + .allOf[1].properties)
            }
        else .
        end
    ) | from_entries
)
