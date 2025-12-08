import { useEffect, useState, useCallback, createContext, useContext, type ReactNode } from 'react';
import { Check, X } from 'lucide-react';

interface ToastMessage {
  id: number;
  message: string;
  type: 'success' | 'error' | 'info';
}

interface ToastContextValue {
  showToast: (message: string, type?: ToastMessage['type']) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

let toastId = 0;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastMessage[]>([]);

  const showToast = useCallback((message: string, type: ToastMessage['type'] = 'success') => {
    const id = ++toastId;
    setToasts((prev) => [...prev, { id, message, type }]);
  }, []);

  const removeToast = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  return (
    <ToastContext.Provider value={{ showToast }}>
      {children}
      <div className="fixed bottom-4 right-4 z-[100] flex flex-col gap-2">
        {toasts.map((toast) => (
          <ToastItem key={toast.id} toast={toast} onClose={() => removeToast(toast.id)} />
        ))}
      </div>
    </ToastContext.Provider>
  );
}

function ToastItem({ toast, onClose }: { toast: ToastMessage; onClose: () => void }) {
  const [isVisible, setIsVisible] = useState(false);

  useEffect(() => {
    // 入场动画
    requestAnimationFrame(() => setIsVisible(true));

    // 自动关闭
    const timer = setTimeout(() => {
      setIsVisible(false);
      setTimeout(onClose, 200); // 等待退场动画
    }, 3000);

    return () => clearTimeout(timer);
  }, [onClose]);

  const bgColor = {
    success: 'bg-success/90',
    error: 'bg-danger/90',
    info: 'bg-accent/90',
  }[toast.type];

  const Icon = toast.type === 'error' ? X : Check;

  return (
    <div
      className={`
        flex items-center gap-2 px-4 py-3 rounded-lg shadow-lg backdrop-blur-sm
        ${bgColor} text-white text-sm font-medium
        transition-all duration-200 ease-out
        ${isVisible ? 'translate-x-0 opacity-100' : 'translate-x-4 opacity-0'}
      `}
    >
      <Icon size={16} />
      <span>{toast.message}</span>
    </div>
  );
}

export function useToast() {
  const context = useContext(ToastContext);
  if (!context) {
    throw new Error('useToast must be used within a ToastProvider');
  }
  return context;
}
