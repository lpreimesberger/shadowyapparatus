//go:build wasm
// +build wasm

package main

import "syscall/js"

// jsArrayToGoArray converts a JavaScript array to a Go interface{} array
func jsArrayToGoArray(jsArray js.Value) []interface{} {
	if jsArray.IsUndefined() || jsArray.IsNull() {
		return []interface{}{}
	}
	
	length := jsArray.Length()
	result := make([]interface{}, length)
	
	for i := 0; i < length; i++ {
		item := jsArray.Index(i)
		if item.Type() == js.TypeObject {
			// Convert JS object to Go map
			result[i] = jsObjectToGoMap(item)
		} else {
			// Convert primitive value
			switch item.Type() {
			case js.TypeString:
				result[i] = item.String()
			case js.TypeNumber:
				result[i] = item.Float()
			case js.TypeBoolean:
				result[i] = item.Bool()
			default:
				result[i] = item.String() // fallback
			}
		}
	}
	
	return result
}

// jsObjectToGoMap converts a JavaScript object to a Go map
func jsObjectToGoMap(jsObj js.Value) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Get all property names (this is a simplified version)
	// For transaction inputs/outputs, we know the expected fields
	if !jsObj.Get("txid").IsUndefined() {
		// This is a transaction input
		result["txid"] = jsObj.Get("txid").String()
		result["vout"] = uint32(jsObj.Get("vout").Float())
		result["script_sig"] = jsObj.Get("script_sig").String()
		result["sequence"] = uint32(jsObj.Get("sequence").Float())
	} else if !jsObj.Get("value").IsUndefined() {
		// This is a transaction output
		result["value"] = uint64(jsObj.Get("value").Float())
		result["script_pubkey"] = jsObj.Get("script_pubkey").String()
		result["address"] = jsObj.Get("address").String()
	}
	
	return result
}