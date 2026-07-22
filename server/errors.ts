export type ErrorCode =
  | "VALIDATION_ERROR"
  | "NOT_FOUND"
  | "INVALID_STATE_TRANSITION"
  | "METHOD_NOT_ALLOWED"
  | "ROUTE_NOT_FOUND"
  | "INVALID_JSON"
  | "PAYLOAD_TOO_LARGE"
  | "PROVIDER_NOT_CONFIGURED"
  | "PROVIDER_UNAVAILABLE"
  | "PROVIDER_REQUEST_FAILED"
  | "PROVIDER_INVALID_RESPONSE"
  | "INTERNAL_ERROR";

export interface ErrorDetail {
  field?: string;
  message: string;
}

export class DomainError extends Error {
  constructor(
    public readonly code: ErrorCode,
    message: string,
    public readonly details?: ErrorDetail[],
  ) {
    super(message);
    this.name = "DomainError";
  }
}

export function isDomainError(error: unknown, code?: ErrorCode): error is DomainError {
  return error instanceof DomainError && (code === undefined || error.code === code);
}

export function errorStatus(code: ErrorCode): number {
  switch (code) {
    case "VALIDATION_ERROR":
    case "INVALID_JSON":
      return 400;
    case "NOT_FOUND":
    case "ROUTE_NOT_FOUND":
      return 404;
    case "INVALID_STATE_TRANSITION":
      return 409;
    case "METHOD_NOT_ALLOWED":
      return 405;
    case "PAYLOAD_TOO_LARGE":
      return 413;
    case "PROVIDER_NOT_CONFIGURED":
      return 503;
    case "PROVIDER_UNAVAILABLE":
    case "PROVIDER_REQUEST_FAILED":
    case "PROVIDER_INVALID_RESPONSE":
      return 502;
    default:
      return 500;
  }
}
