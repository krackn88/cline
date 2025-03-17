import { expect } from "chai";
import { HuggingfaceHandler } from "../../api/providers/huggingface";
import { ApiHandlerOptions } from "../../shared/api";

describe("HuggingfaceHandler", () => {
  const options: ApiHandlerOptions = {
    huggingfaceApiKey: "your-huggingface-api-key",
  };

  const handler = new HuggingfaceHandler(options);

  it("should create a message", async () => {
    const systemPrompt = "Test system prompt";
    const messages = [{ role: "user", content: "Test message" }];

    const stream = handler.createMessage(systemPrompt, messages);
    const result = [];

    for await (const chunk of stream) {
      result.push(chunk);
    }

    expect(result).to.be.an("array").that.is.not.empty;
    expect(result[0]).to.have.property("type", "text");
  });

  it("should get model information", () => {
    const model = handler.getModel();
    expect(model).to.have.property("id", "huggingface");
    expect(model.info).to.have.property("name", "Huggingface");
  });
});
