package deluge

// Casts an []interface{} to []string.
// Assumes all values are of string.
func InterfaceToStringSlice(slice []interface{}) []string {
    result := make([]string, len(slice))

    for i, v := range slice {
        result[i] = v.(string)
    }

    return result
}