package differ

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/RedHatInsights/insights-content-service/content"
	"github.com/stretchr/testify/assert"
)

func makeFetchRequest(testFileName string) (err error, rules types.RulesMap) {
	config, err := conf.LoadConfiguration(configFileEnvVariableName, defaultConfigFileName)
	if err == nil {
		s := httptest.NewServer(http.HandlerFunc(Handler))
		depencenciesConfiguration := conf.GetDependenciesConfiguration(&config)
		depencenciesConfiguration.ContentServiceServer = s.URL
		depencenciesConfiguration.ContentServiceEndpoint = "/tests/content/" + testFileName
		rules, err = fetchAllRulesContent(&depencenciesConfiguration)
	}
	return err, rules
}
func TestFetchAllRulesContentOk(t *testing.T) {
	err, rules := makeFetchRequest("ok")
	assert.NoError(t, err)
	allContentFromMap := getAllContentFromMap(rules)
	assert.NotEmpty(t, allContentFromMap)
	assert.Len(t, allContentFromMap, 2)
}
func TestFetchAllRulesContentNoInternal(t *testing.T) {
	err, rules := makeFetchRequest("no_internal")
	assert.NoError(t, err)
	allContentFromMap := getAllContentFromMap(rules)
	assert.NotEmpty(t, allContentFromMap)
	assert.Len(t, allContentFromMap, 1)
}
func TestFetchAllRulesContentNoExternal(t *testing.T) {
	err, rules := makeFetchRequest("no_external")
	assert.NoError(t, err)
	allContentFromMap := getAllContentFromMap(rules)
	assert.Empty(t, allContentFromMap)
}
func TestFetchAllRulesContentMissing(t *testing.T) {
	err, rules := makeFetchRequest("missing")
	assert.NoError(t, err)
	allContentFromMap := getAllContentFromMap(rules)
	assert.Empty(t, allContentFromMap)
}
func Handler(w http.ResponseWriter, r *http.Request) {
	con, _, _ := content.ParseRuleContentDir("../" + r.RequestURI)
	var temp bytes.Buffer
	enc := gob.NewEncoder(&temp)
	err := enc.Encode(con)
	var tosend string
	if err == nil {
		tosend = temp.String()
		fmt.Fprint(w, tosend)
	}
	fmt.Fprint(w, tosend)
}
