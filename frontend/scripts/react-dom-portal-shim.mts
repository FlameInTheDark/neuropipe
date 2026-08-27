/* SSR test shim: everything from the real react-dom (version, internals that
   react-dom/server cross-checks) except createPortal, which renders portal
   children inline so renderToString can exercise modal content without a
   real DOM container. */
export * from "react-dom";
export const createPortal = (children: React.ReactNode): React.ReactNode => children;
