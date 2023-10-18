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
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

func makeFetchRequest(testFileName string) (rules types.RulesMap, err error) {
	config, err := conf.LoadConfiguration(configFileEnvVariableName, defaultConfigFileName)
	if err != nil {
		return rules, err
	}
	server := httptest.NewServer(http.HandlerFunc(Handler))
	depencenciesConfiguration := conf.GetDependenciesConfiguration(&config)
	depencenciesConfiguration.ContentServiceServer = server.URL
	depencenciesConfiguration.ContentServiceEndpoint = "/tests/content/" + testFileName
	rules, err = fetchAllRulesContent(&depencenciesConfiguration)
	return rules, err
}

// TestFetchAllRulesContentOk cheks if the functon fetchAllRulesContent return valid  RulesMap
func TestFetchAllRulesContentOk(t *testing.T) {
	rules, err := makeFetchRequest("ok")
	assert.NoError(t, err)
	allContentFromMap := getAllContentFromMap(rules)
	assert.NotEmpty(t, allContentFromMap)
	assert.Len(t, allContentFromMap, 2)
}
func TestFetchAllRulesContentNoInternal(t *testing.T) {
	rules, err := makeFetchRequest("no_internal")
	assert.NoError(t, err)
	allContentFromMap := getAllContentFromMap(rules)
	assert.NotEmpty(t, allContentFromMap)
	assert.Len(t, allContentFromMap, 1)
}
func TestFetchAllRulesContentNoExternal(t *testing.T) {
	rules, err := makeFetchRequest("no_external")
	assert.NoError(t, err)
	allContentFromMap := getAllContentFromMap(rules)
	assert.Empty(t, allContentFromMap)
}
func TestFetchAllRulesContentMissing(t *testing.T) {
	rules, err := makeFetchRequest("missing")
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
	if err != nil {
		return
	}
	tosend = temp.String()
	_, err = fmt.Fprint(w, tosend)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
}
