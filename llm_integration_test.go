//go:build integration
// +build integration

package main

import (
	"fmt"
	"net/http"
	"testing"
)

const titleGenerationPrompt = `
Return possible titles for this document, the titles should be describe the content of this document.

- Keep the language of the original document.
- Return this result as a json array. 
- You must respond with only valid JSON. 
- No markdown, only json.
- In the case the document seems to be not correctly interpreted, return "RESCAN DOCUMENT".

--- document ocr content (first {truncate_chars} chars) ---

{content}`

const contentSample = `
16 April 1963
My Dear Fellow Clergymen:
While confined here in the Birmingham city jail, I came across your recent statement calling my present activities "unwise and untimely." Seldom do I pause to answer criticism of my work and ideas. If I sought to answer all the criticisms that cross my desk, my secretaries would have little time for anything other than such correspondence in the course of the day, and I would have no time for constructive work. But since I feel that you are men of genuine good will and that your criticisms are sincerely set forth, I want to try to answer your statement in what I hope will be patient and reasonable terms.

I think I should indicate why I am here in Birmingham, since you have been influenced by the view which argues against "outsiders coming in." I have the honor of serving as president of the Southern Christian Leadership Conference, an organization operating in every southern state, with headquarters in Atlanta, Georgia. We have some eighty five affiliated organizations across the South, and one of them is the Alabama Christian Movement for Human Rights. Frequently we share staff, educational and financial resources with our affiliates. Several months ago the affiliate here in Birmingham asked us to be on call to engage in a nonviolent direct action program if such were deemed necessary. We readily consented, and when the hour came we lived up to our promise. So I, along with several members of my staff, am here because I was invited here. I am here because I have organizational ties here.

But more basically, I am in Birmingham because injustice is here. Just as the prophets of the eighth century B.C. left their villages and carried their "thus saith the Lord" far beyond the boundaries of their home towns, and just as the Apostle Paul left his village of Tarsus and carried the gospel of Jesus Christ to the far corners of the Greco Roman world, so am I compelled to carry the gospel of freedom beyond my own home town. Like Paul, I must constantly respond to the Macedonian call for aid.

Moreover, I am cognizant of the interrelatedness of all communities and states. I cannot sit idly by in Atlanta and not be concerned about what happens in Birmingham. Injustice anywhere is a threat to justice everywhere. We are caught in an inescapable network of mutuality, tied in a single garment of destiny. Whatever affects one directly, affects all indirectly. Never again can we afford to live with the narrow, provincial "outside agitator" idea. Anyone who lives inside the United States can never be considered an outsider anywhere within its bounds.

You deplore the demonstrations taking place in Birmingham. But your statement, I am sorry to say, fails to express a similar concern for the conditions that brought about the demonstrations. I am sure that none of you would want to rest content with the superficial kind of social analysis that deals merely with effects and does not grapple with underlying causes. It is unfortunate that demonstrations are taking place in Birmingham, but it is even more unfortunate that the city's white power structure left the Negro community with no alternative.
`

func TestLLMClient_GenerateTitleFromContent(t *testing.T) {
	// Create a config for testing
	config := &Config{
		LLM: LLMConfig{
			API: struct {
				BaseURL  string `yaml:"base_url"`
				Endpoint string `yaml:"endpoint"`
			}{
				BaseURL:  "http://localhost:8080",
				Endpoint: "/v1/chat/completions",
			},
			Models: struct {
				TitleGeneration   string `yaml:"title_generation"`
				ContentExtraction string `yaml:"content_extraction"`
			}{
				TitleGeneration:   "ibnzterrell/Meta-Llama-3.3-70B-Instruct-AWQ-INT4",
				ContentExtraction: "google/gemma-3-27b-it",
			},
			Prompts: struct {
				TitleGeneration   string `yaml:"title_generation"`
				ContentExtraction string `yaml:"content_extraction"`
			}{
				TitleGeneration: titleGenerationPrompt,
			},
		},
		Processing: ProcessingConfig{
			TitleGeneration: struct {
				TruncateCharactersOfContent int `yaml:"truncate_characters_of_content"`
			}{
				TruncateCharactersOfContent: 2000,
			},
		},
	}

	type fields struct {
		config     *Config
		httpClient *http.Client
	}
	type args struct {
		content string
	}
	tests := []struct {
		name         string
		fields       fields
		args         args
		wantMinCount int
		wantErr      bool
	}{
		{
			name: "Generate titles from Martin Luther King Jr. letter content",
			fields: fields{
				config:     config,
				httpClient: &http.Client{},
			},
			args: args{
				content: contentSample,
			},
			wantMinCount: 3, // Expect at least three titles to be generated
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &LLMClient{
				config:     tt.fields.config,
				httpClient: tt.fields.httpClient,
			}
			got, err := c.GenerateTitleFromContent(tt.args.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("LLMClient.GenerateTitleFromContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) < tt.wantMinCount {
				t.Errorf("LLMClient.GenerateTitleFromContent() returned %d captions, want at least %d", len(got), tt.wantMinCount)
			}
			if len(got) > 0 {
				titles := make([]string, len(got))
				for i, caption := range got {
					titles[i] = fmt.Sprintf("%s (score: %.2f)", caption.Caption, caption.Score)
				}
				t.Logf("Generated captions: %v", titles)
			}
		})
	}
}
