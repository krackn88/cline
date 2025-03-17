import axios from "axios";
import { ApiHandler } from "..";
import { ApiHandlerOptions, ModelInfo } from "../../shared/api";
import { ApiStream } from "../transform/stream";

export class VastAiHandler implements ApiHandler {
  private options: ApiHandlerOptions;
  private apiUrl: string;
  private apiKey: string;

  constructor(options: ApiHandlerOptions) {
    this.options = options;
    this.apiUrl = options.vastAiApiUrl || "https://api.vast.ai";
    this.apiKey = options.vastAiApiKey || "";
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

      const response = await axios.post(`${this.apiUrl}/v1/models/${model.id}/generate`, request, {
        headers: {
          Authorization: `Bearer ${this.apiKey}`,
          "Content-Type": "application/json",
        },
      });

      if (response.status !== 200) {
        throw new Error(`Vast.ai API error: ${response.statusText}`);
      }

      const result = response.data;

      if (!result || !result.generated_text) {
        throw new Error("No content in Vast.ai response");
      }

      yield {
        type: "text",
        text: result.generated_text,
      };
    } catch (error) {
      if (error instanceof Error) {
        throw new Error(`Vast.ai request failed: ${error.message}`);
      }
    }
  }

  getModel(): { id: string; info: ModelInfo } {
    return {
      id: "vastai",
      info: {
        name: "Vast.ai",
        description: "Vast.ai API integration",
        maxTokens: 1000,
      },
    };
  }
}
