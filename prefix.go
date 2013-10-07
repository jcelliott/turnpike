// Copyright (c) 2013 Joshua Elliott
// Released under the MIT License
// http://opensource.org/licenses/MIT

package turnpike

import (
	"fmt"
	"net/url"
	"strings"
)

type prefixMap map[string]string

func (pm prefixMap) registerPrefix(prefix, URI string) error {
	if _, err := url.ParseRequestURI(URI); err != nil {
		return fmt.Errorf("Invalid URI: %s", URI)
	}
	pm[prefix] = URI
	return nil
}

// returns the full URI that the curie represents
func (pm prefixMap) resolveCurie(curie string) (string, error) {
	parts := strings.SplitN(curie, ":", 2)
	// if there is no reference, return the URI for the prefix
	if len(parts) < 2 {
		if URI, ok := pm[parts[0]]; ok {
			return URI, nil
		}
		return "", fmt.Errorf("Unable to resolve curie: %s", curie)
	}
	// with a reference, append the ref to the URI
	if URI, ok := pm[parts[0]]; ok {
		return URI + parts[1], nil
	}
	return "", fmt.Errorf("Unable to resolve curie: %s", curie)
}

// convenience function that will resolve a curie and pass through a URI
func checkCurie(pm prefixMap, curie string) string {
	if pm == nil {
		// no prefixes defined for this client, curie is a uri
		return curie
	}
	uri, err := pm.resolveCurie(curie)
	if err != nil {
		// not registered for this client or it's a uri
		return curie
	}
	// curie registered for this client, return real uri
	return uri
}
