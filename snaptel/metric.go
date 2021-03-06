/*
http://www.apache.org/licenses/LICENSE-2.0.txt


Copyright 2017 Intel Corporation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package snaptel

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/intelsdi-x/snap-client-go/client/plugins"
	"github.com/intelsdi-x/snap-client-go/models"
	"github.com/urfave/cli"
)

func listMetrics(ctx *cli.Context) error {
	verbose := ctx.Bool("verbose")

	metrics, err := queryMetrics(ctx)
	if err != nil {
		return err
	}

	/*
		NAMESPACE               VERSION
		/intel/mock/foo         1,2
		/intel/mock/bar         1
	*/
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', 0)

	if verbose {

		// NAMESPACE                VERSION         UNIT          DESCRIPTION
		// /intel/mock/foo           1
		//  /intel/mock/foo          2               mock unit     mock description
		//  /intel/mock/[host]/baz   2               mock unit     mock description

		printFields(w, false, 0, "NAMESPACE", "VERSION", "UNIT", "DESCRIPTION")
		for _, mt := range metrics {
			namespace := getNamespace(mt)
			printFields(w, false, 0, namespace, mt.Version, mt.Unit, mt.Description)
		}
		w.Flush()
		return nil
	}

	// groups the same namespace of different versions.
	metsByVer := make(map[string][]string)
	for _, mt := range metrics {
		metsByVer[*mt.Namespace] = append(metsByVer[*mt.Namespace], strconv.Itoa(int(mt.Version)))
	}
	//make list in alphabetical order
	var key []string
	for k := range metsByVer {
		key = append(key, k)
	}
	sort.Strings(key)

	printFields(w, false, 0, "NAMESPACE", "VERSIONS")
	for _, ns := range key {
		printFields(w, false, 0, ns, strings.Join(metsByVer[ns], ","))
	}
	w.Flush()
	return nil
}

func printMetric(metric *models.Metric, idx int) error {
	/*
		NAMESPACE                VERSION         LAST ADVERTISED TIME
		/intel/mock/foo          2               Wed, 09 Sep 2015 10:01:04 PDT

		  Rules for collecting /intel/mock/foo:

		     NAME        TYPE            DEFAULT         REQUIRED     MINIMUM   MAXIMUM
		     name        string          bob             false
		     password    string                          true
		     portRange   int                             false        9000      10000
	*/

	namespace := getNamespace(metric)

	if idx > 0 {
		fmt.Printf("\n")
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', 0)
	printFields(w, false, 0, "NAMESPACE", "VERSION", "UNIT", "LAST ADVERTISED TIME", "DESCRIPTION")
	printFields(w, false, 0, namespace, metric.Version, metric.Unit, time.Unix(metric.LastAdvertisedTimestamp, 0).Format(time.RFC1123), metric.Description)
	w.Flush()
	if metric.Dynamic {

		//	NAMESPACE                VERSION     UNIT        LAST ADVERTISED TIME            DESCRIPTION
		//	/intel/mock/[host]/baz   2           mock unit   Wed, 09 Sep 2015 10:01:04 PDT   mock description
		//
		//	  Dynamic elements of namespace: /intel/mock/[host]/baz
		//
		//           NAME        DESCRIPTION
		//           host        name of the host
		//
		//	  Rules for collecting /intel/mock/[host]/baz:
		//
		//	     NAME        TYPE            DEFAULT         REQUIRED     MINIMUM   MAXIMUM

		fmt.Printf("\n  Dynamic elements of namespace: %s\n\n", namespace)
		printFields(w, true, 6, "NAME", "DESCRIPTION")
		for _, v := range metric.DynamicElements {
			printFields(w, true, 6, v.Name, v.Description)
		}
		w.Flush()
	}
	fmt.Printf("\n  Rules for collecting %s:\n\n", namespace)
	printFields(w, true, 6, "NAME", "TYPE", "DEFAULT", "REQUIRED", "MINIMUM", "MAXIMUM")
	for _, rule := range metric.Policy {
		printFields(w, true, 6, rule.Name, rule.Type, rule.Default, rule.Required, rule.Minimum, rule.Maximum)
	}
	w.Flush()
	return nil
}

func getMetric(ctx *cli.Context) error {
	if !ctx.IsSet("metric-namespace") {
		return newUsageError("Error: Must provide metric namespace", ctx)
	}
	metrics, err := queryMetrics(ctx)
	if err != nil {
		return err
	}

	for i, m := range metrics {
		err := printMetric(m, i)
		if err != nil {
			return err
		}
	}
	return nil
}

func getNamespace(mt *models.Metric) string {
	ns := mt.Namespace
	if mt.Dynamic {
		fc := GetFirstChar(*ns)
		slice := strings.Split(*ns, fc)
		for _, v := range mt.DynamicElements {
			slice[v.Index+1] = "[" + *v.Name + "]"
		}
		*ns = strings.Join(slice, fc)
	}
	return *ns
}

func queryMetrics(ctx *cli.Context) ([]*models.Metric, error) {
	ns := ctx.String("metric-namespace")
	ver := ctx.Int("metric-version")
	params := plugins.NewGetMetricsParamsWithTimeout(FlTimeout.Value)

	if strings.Trim(ns, " ") != "" {
		params.SetNs(&ns)
	}
	if ver > 0 {
		ver64 := int64(ver)
		params.SetVer(&ver64)
	}

	resp, err := client.Plugins.GetMetrics(params, authInfoWriter)
	if err != nil {
		return nil, getErrorDetail(err, ctx)
	}

	if (len(ns) > 0 || ver > 0) && len(resp.Payload.Metrics) == 0 {
		return nil, fmt.Errorf("No metric found the giving namespace %s, version %d", ns, ver)
	} else if len(resp.Payload.Metrics) == 0 {
		return nil, fmt.Errorf("No metrics found. Have you loaded any collectors yet?")
	}
	return resp.Payload.Metrics, nil
}
