// The MIT License (MIT)

// Copyright (c) 2013 Joshua Elliott

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

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
