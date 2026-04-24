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
