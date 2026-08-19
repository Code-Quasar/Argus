package cmd

import (
	"Argus/api"
	"Argus/capacity"
	"fmt"
	"net/http"
	"strconv"

	"github.com/spf13/cobra"
)

var servePort int

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Hydra gateway server (the OpenAI-compatible endpoint + admin API)",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := OpenDefault()
		if err != nil {
			return err
		}

		keysByProvider := make(map[string][]string)
		for _, key := range store.ListKeys("") {
			keysByProvider[key.Provider] = append(keysByProvider[key.Provider], key.Key)
		}

		providers := make([]api.Provider, 0, len(keysByProvider))
		endpoints := map[string]string{
			capacity.Gemini:     capacity.EndpointGemini,
			capacity.Groq:       capacity.EndpointGroq,
			capacity.Mistral:    capacity.EndpointMistral,
			capacity.Cerebras:   capacity.EndpointCerebras,
			capacity.OpenRouter: capacity.EndpointOpenRouter,
			capacity.Zhipu:      capacity.EndpointZhipu,
		}
		for provider, keys := range keysByProvider {
			endpoint, ok := endpoints[provider]
			if !ok {
				return fmt.Errorf("provider %q has no configured endpoint", provider)
			}
			style := api.StyleOpenAI
			if provider == capacity.Gemini {
				style = api.StyleGemini
			}
			providers = append(providers, api.Provider{
				Name:     provider,
				Endpoint: endpoint,
				Style:    style,
				Keys:     keys,
			})
		}

		models := make([]api.ModelRoute, 0, len(capacity.Catalog.ModelLimits))
		for _, model := range capacity.Catalog.ModelLimits {
			models = append(models, api.ModelRoute{Model: model.Model, Provider: model.Provider})
		}

		gateway, err := api.New(api.Config{Providers: providers, Models: models})
		if err != nil {
			return err
		}

		address := ":" + strconv.Itoa(servePort)
		fmt.Printf("Starting Argus on %s...\n", address)
		return http.ListenAndServe(address, gateway.Routes())
	},
}

func init() {
	serveCmd.Flags().IntVar(&servePort, "port", 8080, "port to listen on")
	rootCmd.AddCommand(serveCmd)
}
