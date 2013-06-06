package turnpike

import (
	"fmt"
	"net/url"
	"strings"
)

type PrefixMap map[string]string

func (pm PrefixMap) RegisterPrefix(prefix, URI string) error {
	if _, err := url.ParseRequestURI(URI); err != nil {
		return fmt.Errorf("Invalid URI: %s", URI)
	}
	pm[prefix] = URI
	return nil
}

// returns the full URI that the curie represents
func (pm PrefixMap) ResolveCurie(curie string) (string, error) {
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
func CheckCurie(pm PrefixMap, curie string) string {
	if pm == nil {
		// no prefixes defined for this client, curie is a uri
		return curie
	}
	uri, err := pm.ResolveCurie(curie)
	if err != nil {
		// not registered for this client or it's a uri
		return curie
	}
	// curie registered for this client, return real uri
	return uri
}
