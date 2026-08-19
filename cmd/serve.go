package cmd

import (
	"Argus/api"
	"Argus/capacity"
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var servePort int
var serveBackground bool
var serveDaemon bool

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Argus gateway server (the OpenAI-compatible endpoint + admin API)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if serveBackground && serveDaemon {
			return fmt.Errorf("cannot combine --background and internal daemon mode")
		}
		if serveBackground {
			return startBackground()
		}

		var store *Store
		var err error
		if serveDaemon {
			password, readErr := bufio.NewReader(os.Stdin).ReadString('\n')
			if readErr != nil && len(password) == 0 {
				return fmt.Errorf("read server password: %w", readErr)
			}
			password = strings.TrimSuffix(password, "\n")
			password = strings.TrimSuffix(password, "\r")
			store, err = OpenWithPassword(password)
		} else {
			store, err = OpenDefault()
		}
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
			if store.IsPaused(model.Provider, model.Model) {
				continue
			}
			models = append(models, api.ModelRoute{
				Model:    model.Model,
				Provider: model.Provider,
				Priority: store.Priority(model.Provider, model.Model),
			})
		}

		gateway, err := api.New(api.Config{Providers: providers, Models: models})
		if err != nil {
			return err
		}

		address := ":" + strconv.Itoa(servePort)
		fmt.Printf("Starting Argus on %s...\n", address)
		if serveDaemon {
			pidPath, err := DefaultPIDPath()
			if err != nil {
				return err
			}
			defer os.Remove(pidPath)
		}
		return http.ListenAndServe(address, gateway.Routes())
	},
}

func init() {
	serveCmd.Flags().IntVar(&servePort, "port", 8080, "port to listen on")
	serveCmd.Flags().BoolVar(&serveBackground, "background", false, "run the server as a detached background process")
	serveCmd.Flags().BoolVar(&serveDaemon, "daemon", false, "internal detached server mode")
	_ = serveCmd.Flags().MarkHidden("daemon")
	rootCmd.AddCommand(serveCmd)
}
