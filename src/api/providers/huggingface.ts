import { ApiHandler } from "..";
import { ApiHandlerOptions, ModelInfo } from "../../shared/api";
import { ApiStream } from "../transform/stream";
import axios from "axios";

export class HuggingfaceHandler implements ApiHandler {
  private options: ApiHandlerOptions;
  private apiUrl: string;
  private apiKey: string;

  constructor(options: ApiHandlerOptions) {
    this.options = options;
    this.apiUrl = options.huggingfaceApiUrl || "https://api-inference.huggingface.co/models";
    this.apiKey = options.huggingfaceApiKey || "";
  }

  async *createMessage(systemPrompt: string, messages: any[]): ApiStream {
    try {
      const model = this.getModel();

      const formattedMessages = messages.map((msg) => {
        const content = Array.isArray(msg.content)
          ? msg.content.map((block) => ("text" in block ? block.text : "")).join("")
          : msg.content;

        return {
          role: msg.role,
          content: content,
        };
      });

      const request = {
        inputs: {
          system_prompt: systemPrompt,
          messages: formattedMessages,
        },
        model: model.id,
      };

      const response = await axios.post(`${this.apiUrl}/${model.id}`, request, {
        headers: {
          Authorization: `Bearer ${this.apiKey}`,
          "Content-Type": "application/json",
        },
      });

      if (response.status !== 200) {
        throw new Error(`Huggingface API error: ${response.statusText}`);
      }

      const result = response.data;

      if (!result || !result.generated_text) {
        throw new Error("No content in Huggingface response");
      }

      yield {
        type: "text",
        text: result.generated_text,
      };
    } catch (error) {
      if (error instanceof Error) {
        throw new Error(`Huggingface request failed: ${error.message}`);
      }
    }
  }

  getModel(): { id: string; info: ModelInfo } {
    return {
      id: "huggingface",
      info: {
        name: "Huggingface",
        description: "Huggingface API integration",
        maxTokens: 1000,
      },
    };
  }
}
