import { expect } from "chai";
import { GitHubHandler } from "../../api/providers/github";
import { ApiHandlerOptions } from "../../shared/api";

describe("GitHubHandler", () => {
  const options: ApiHandlerOptions = {
    githubToken: "your-github-token",
  };

  const handler = new GitHubHandler(options);

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
    expect(model).to.have.property("id", "github");
    expect(model.info).to.have.property("name", "GitHub");
  });
});
