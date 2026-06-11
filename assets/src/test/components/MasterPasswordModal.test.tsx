import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/preact";
import { MasterPasswordModal } from "../../components/MasterPasswordModal";

vi.mock("../../utils/apiClientSingleton", () => ({
  getAPIClient: vi.fn(),
}));

import { getAPIClient } from "../../utils/apiClientSingleton";

const mockApiClient = {
  setMasterPassword: vi.fn(),
  unlockCredentials: vi.fn(),
  getMasterPasswordStatus: vi.fn(),
};

describe("MasterPasswordModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useRealTimers();
    (getAPIClient as ReturnType<typeof vi.fn>).mockReturnValue(mockApiClient);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("rendering", () => {
    it("does not render when isOpen is false", () => {
      render(<MasterPasswordModal isOpen={false} mode="setup" onSuccess={vi.fn()} onClose={vi.fn()} />);
      expect(document.querySelector('[style*="position: fixed"]')).toBeNull();
    });

    it("renders setup mode correctly", async () => {
      render(<MasterPasswordModal isOpen={true} mode="setup" onSuccess={vi.fn()} onClose={vi.fn()} />);

      await waitFor(() => {
        expect(screen.getByText("设置主密码")).toBeTruthy();
        expect(screen.getByPlaceholderText("输入密码")).toBeTruthy();
        expect(screen.getByPlaceholderText("确认密码")).toBeTruthy();
      });
    });

    it("renders unlock mode correctly", async () => {
      render(<MasterPasswordModal isOpen={true} mode="unlock" onSuccess={vi.fn()} onClose={vi.fn()} />);

      await waitFor(() => {
        expect(screen.getByText("解锁凭证")).toBeTruthy();
        expect(screen.getByPlaceholderText("输入密码")).toBeTruthy();
        expect(screen.queryByPlaceholderText("确认密码")).toBeNull();
      });
    });

    it("renders cancel and submit buttons", async () => {
      render(<MasterPasswordModal isOpen={true} mode="setup" onSuccess={vi.fn()} onClose={vi.fn()} />);

      await waitFor(() => {
        expect(screen.getByText("取消")).toBeTruthy();
        expect(screen.getByText("确定")).toBeTruthy();
      });
    });
  });

  describe("validation", () => {
    it("shows error when password is too short", async () => {
      render(<MasterPasswordModal isOpen={true} mode="setup" onSuccess={vi.fn()} onClose={vi.fn()} />);

      await waitFor(() => {
        expect(screen.getByPlaceholderText("输入密码")).toBeTruthy();
      });

      const passwordInput = screen.getByPlaceholderText("输入密码");
      fireEvent.input(passwordInput, { target: { value: "123" } });

      const submitButton = screen.getByText("确定");
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText("密码至少4个字符")).toBeTruthy();
      });
    });

    it("shows error when passwords do not match in setup mode", async () => {
      render(<MasterPasswordModal isOpen={true} mode="setup" onSuccess={vi.fn()} onClose={vi.fn()} />);

      const passwordInput = screen.getByPlaceholderText("输入密码");
      const confirmInput = screen.getByPlaceholderText("确认密码");

      fireEvent.input(passwordInput, { target: { value: "password123" } });
      fireEvent.input(confirmInput, { target: { value: "differentpassword" } });

      const submitButton = screen.getByText("确定");
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText("两次密码不一致")).toBeTruthy();
      });
    });

    it("does not show confirm password field in unlock mode", async () => {
      render(<MasterPasswordModal isOpen={true} mode="unlock" onSuccess={vi.fn()} onClose={vi.fn()} />);

      expect(screen.queryByPlaceholderText("确认密码")).toBeNull();
    });
  });

  describe("API calls", () => {
    it("calls setMasterPassword and succeeds in setup mode", async () => {
      mockApiClient.setMasterPassword.mockResolvedValue({ success: true });
      const onSuccess = vi.fn();

      render(<MasterPasswordModal isOpen={true} mode="setup" onSuccess={onSuccess} onClose={vi.fn()} />);

      const passwordInput = screen.getByPlaceholderText("输入密码");
      const confirmInput = screen.getByPlaceholderText("确认密码");

      fireEvent.input(passwordInput, { target: { value: "password123" } });
      fireEvent.input(confirmInput, { target: { value: "password123" } });

      const submitButton = screen.getByText("确定");
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(mockApiClient.setMasterPassword).toHaveBeenCalledWith("password123");
      });

      await waitFor(() => {
        expect(onSuccess).toHaveBeenCalled();
      });
    });

    it("calls unlockCredentials and succeeds in unlock mode", async () => {
      mockApiClient.unlockCredentials.mockResolvedValue({ success: true, locked_until: 0 });
      const onSuccess = vi.fn();

      render(<MasterPasswordModal isOpen={true} mode="unlock" onSuccess={onSuccess} onClose={vi.fn()} />);

      const passwordInput = screen.getByPlaceholderText("输入密码");
      fireEvent.input(passwordInput, { target: { value: "password123" } });

      const submitButton = screen.getByText("确定");
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(mockApiClient.unlockCredentials).toHaveBeenCalledWith("password123");
      });

      await waitFor(() => {
        expect(onSuccess).toHaveBeenCalled();
      });
    });

    it("shows error when API returns error in setup mode", async () => {
      mockApiClient.setMasterPassword.mockResolvedValue({ success: false, error: "密码已存在" });

      render(<MasterPasswordModal isOpen={true} mode="setup" onSuccess={vi.fn()} onClose={vi.fn()} />);

      const passwordInput = screen.getByPlaceholderText("输入密码");
      const confirmInput = screen.getByPlaceholderText("确认密码");

      fireEvent.input(passwordInput, { target: { value: "password123" } });
      fireEvent.input(confirmInput, { target: { value: "password123" } });

      const submitButton = screen.getByText("确定");
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(mockApiClient.setMasterPassword).toHaveBeenCalled();
      });

      await waitFor(() => {
        expect(screen.getByText("密码已存在")).toBeTruthy();
      });
    });

    it("shows error when API returns error in unlock mode", async () => {
      mockApiClient.unlockCredentials.mockResolvedValue({ success: false, error: "密码错误" });

      render(<MasterPasswordModal isOpen={true} mode="unlock" onSuccess={vi.fn()} onClose={vi.fn()} />);

      const passwordInput = screen.getByPlaceholderText("输入密码");
      fireEvent.input(passwordInput, { target: { value: "wrongpassword" } });

      const submitButton = screen.getByText("确定");
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(mockApiClient.unlockCredentials).toHaveBeenCalledWith("wrongpassword");
      });

      await waitFor(() => {
        expect(screen.getByText("密码错误")).toBeTruthy();
      });
    });
  });

  describe("loading state", () => {
    it("disables submit button during loading", async () => {
      mockApiClient.setMasterPassword.mockImplementation(
        () => new Promise((resolve) => setTimeout(() => resolve({ success: true }), 100))
      );

      render(<MasterPasswordModal isOpen={true} mode="setup" onSuccess={vi.fn()} onClose={vi.fn()} />);

      const passwordInput = screen.getByPlaceholderText("输入密码");
      const confirmInput = screen.getByPlaceholderText("确认密码");

      fireEvent.input(passwordInput, { target: { value: "password123" } });
      fireEvent.input(confirmInput, { target: { value: "password123" } });

      const submitButton = screen.getByText("确定");
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect((submitButton as HTMLButtonElement).disabled).toBe(true);
      });
    });
  });

  describe("close", () => {
    it("calls onClose when cancel button is clicked", async () => {
      const onClose = vi.fn();

      render(<MasterPasswordModal isOpen={true} mode="setup" onSuccess={vi.fn()} onClose={onClose} />);

      const cancelButton = screen.getByText("取消");
      fireEvent.click(cancelButton);

      expect(onClose).toHaveBeenCalled();
    });

    it("calls onClose when backdrop is clicked", async () => {
      const onClose = vi.fn();

      render(<MasterPasswordModal isOpen={true} mode="setup" onSuccess={vi.fn()} onClose={onClose} />);

      const backdrop = document.querySelector('[style*="position: fixed"]');
      fireEvent.click(backdrop!);

      expect(onClose).toHaveBeenCalled();
    });

    it("does not call onClose when modal content is clicked", async () => {
      const onClose = vi.fn();

      render(<MasterPasswordModal isOpen={true} mode="setup" onSuccess={vi.fn()} onClose={onClose} />);

      const modalContent = document.querySelector('[style*="border-radius"]');
      fireEvent.click(modalContent!);

      expect(onClose).not.toHaveBeenCalled();
    });
  });
});
