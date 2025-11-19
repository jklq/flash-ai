import type { ChatMessage } from "../types";

const OPENROUTER_API_URL = "https://openrouter.ai/api/v1/chat/completions";
const MODEL = "openai/gpt-oss-120b";

export interface StreamCallbacks {
  onToken: (token: string) => void;
  onComplete: () => void;
  onError: (error: Error) => void;
}

export async function sendChatMessage(
  messages: ChatMessage[],
  callbacks: StreamCallbacks
): Promise<void> {
  const apiKey = localStorage.getItem("openrouter_api_key");

  if (!apiKey) {
    callbacks.onError(
      new Error(
        "OpenRouter API key not found. Please enter your API key in the chat window."
      )
    );
    return;
  }

  try {
    const response = await fetch(OPENROUTER_API_URL, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${apiKey}`,
        "Content-Type": "application/json",
        "HTTP-Referer": window.location.origin,
        "X-Title": "Flash AI Study Assistant",
      },
      body: JSON.stringify({
        model: MODEL,
        messages: messages,
        stream: true,
      }),
    });

    if (!response.ok) {
      throw new Error(
        `OpenRouter API error: ${response.status} ${response.statusText}`
      );
    }

    const reader = response.body?.getReader();
    if (!reader) {
      throw new Error("No response body");
    }

    const decoder = new TextDecoder();
    let buffer = "";

    while (true) {
      const { done, value } = await reader.read();

      if (done) {
        callbacks.onComplete();
        break;
      }

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split("\n");
      buffer = lines.pop() || "";

      for (const line of lines) {
        if (line.startsWith("data: ")) {
          const data = line.slice(6);

          if (data === "[DONE]") {
            callbacks.onComplete();
            return;
          }

          try {
            const parsed = JSON.parse(data);
            const token = parsed.choices?.[0]?.delta?.content;

            if (token) {
              callbacks.onToken(token);
            }
          } catch (e) {
            // Ignore JSON parse errors for partial data
            console.warn("Failed to parse SSE data:", e);
          }
        }
      }
    }
  } catch (error) {
    callbacks.onError(
      error instanceof Error ? error : new Error(String(error))
    );
  }
}
