import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { getAPIClient, resetAPIClient } from "../../utils/apiClientSingleton";

vi.mock("../../utils/apiClient", () => ({
  APIClient: vi.fn().mockImplementation(() => ({})),
}));

describe("apiClientSingleton", () => {
  beforeEach(() => {
    resetAPIClient();
    vi.clearAllMocks();
  });

  afterEach(() => {
    resetAPIClient();
  });

  it("getAPIClient returns an instance", () => {
    const client = getAPIClient();
    expect(client).toBeTruthy();
  });

  it("getAPIClient returns the same instance (singleton)", () => {
    const client1 = getAPIClient();
    const client2 = getAPIClient();
    expect(client1).toBe(client2);
  });

  it("resetAPIClient allows creating a new instance", () => {
    const client1 = getAPIClient();
    resetAPIClient();
    const client2 = getAPIClient();
    expect(client1).not.toBe(client2);
  });

  it("multiple calls return same instance until reset", () => {
    const client1 = getAPIClient();
    const client2 = getAPIClient();
    const client3 = getAPIClient();
    expect(client1).toBe(client2);
    expect(client2).toBe(client3);
  });
});