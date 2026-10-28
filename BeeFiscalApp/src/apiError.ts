/**
 * ApiError — typed error from an API call.
 *
 * - `status`: HTTP status code (0 if network failure before response)
 * - `code`: application-level error code from problem+json or response body
 * - `retryable`: true for network failures and 5xx; false for 4xx client errors
 */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly retryable: boolean;

  constructor(status: number, code: string, retryable: boolean, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.retryable = retryable;
  }
}

/**
 * Parse a fetch Response into an ApiError.
 *
 * For `application/problem+json` responses, extracts `type` and `title` from
 * the body. For other error responses, uses the raw text as the message.
 *
 * A response is retryable when:
 * - status === 0 (network/timeout before response)
 * - status >= 500
 *
 * It is non-retryable when status is 4xx (client error).
 */
export async function parseApiError(response: Response): Promise<ApiError> {
  const status = response.status;
  const retryable = status === 0 || status >= 500;
  let code = `HTTP_${status}`;
  let message = `HTTP ${status}`;

  try {
    const ct = response.headers?.get?.("content-type") ?? "";
    const text = await response.text();
    if (ct.includes("application/problem+json") || text.trim().startsWith("{")) {
      try {
        const body = JSON.parse(text);
        if (typeof body.error === "string") code = body.error;
        else if (typeof body.type === "string") code = body.type;
        else if (typeof body.code === "string") code = body.code;
        if (typeof body.title === "string") message = body.title;
        else if (typeof body.message === "string") message = body.message;
        else message = text;
      } catch {
        message = text;
      }
    } else {
      message = text || message;
    }
  } catch {
    // ignore body read errors
  }

  return new ApiError(status, code, retryable, message);
}

/**
 * Create an ApiError for a network-level failure (no HTTP response).
 */
export function networkApiError(cause: unknown): ApiError {
  const msg =
    cause instanceof Error ? cause.message : String(cause);
  return new ApiError(0, "NETWORK_ERROR", true, msg);
}
