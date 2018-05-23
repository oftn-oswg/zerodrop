// https://w3c.github.io/ServiceWorker/#extendableevent-interface
interface ExtendableEvent extends Event {
  waitUntil(f: Promise<any>): void;
}

// https://w3c.github.io/ServiceWorker/#fetchevent-interface
interface FetchEvent extends ExtendableEvent {
  readonly request: Request;
  readonly preloadResponse: Promise<any>;
  readonly clientId: string;
  readonly reservedClientId: string;
  readonly targetClientId: string;

  respondWith(r: Promise<Response>): void;
}

const is_worker = !self.document;
const mime_image = 'image/png';

// Window load promise
const window_load = new Promise((resolve: (value?: any) => void): void => {
  window.addEventListener('load', resolve);
});

// Constants
const ns = 'http://www.w3.org/1999/xhtml';
const mime_text_regex = /^\s*(?:text\/plain)\s*(?:$|;)/i;
const size = 256; // Anything larger will causes problems in Google Chrome
const pixelgrid = 16;
const self_uri = document.currentScript.getAttribute('src');
const service_worker_container = navigator.serviceWorker;

// Elements
const canvas = document.createElementNS(ns, 'canvas') as HTMLCanvasElement;
const link = document.createElementNS(ns, 'link') as HTMLLinkElement;
const context = canvas.getContext('2d');

// Function
export default function set_emoji_favicon(emoji: any, cacheWithServiceWorker?: any): void {
  // Normalize arguments
  const char = String(emoji) || '';
  const cache = Boolean(cacheWithServiceWorker);

  // Calculate sizing
  const metric = context.measureText(char);
  const iconsize = metric.width;
  const center = (size + size / pixelgrid) / 2;

  const scale = Math.min(size / iconsize, 1);
  const center_scaled = center / scale;

  // Draw emoji
  context.clearRect(0, 0, size, size);
  context.save();
  context.scale(scale, scale);
  context.fillText(char, center_scaled, center_scaled);
  context.restore();

  // Update favicon element
  link.href = canvas.toDataURL(mime_image);
  document.getElementsByTagName('head')[0].appendChild(link);

  // Add favicon to cache
  if (cache && service_worker_container) {
    canvas.toBlob((blob: Blob): void => {
      const reader = new FileReader();
      reader.addEventListener('loadend', () => {
        const array_buffer = reader.result;
        // https://developers.google.com/web/fundamentals/primers/service-workers/registration
        window_load.then(() => {
          service_worker_container.register(self_uri, { scope: '/' });
          service_worker_container.ready.then((registration: ServiceWorkerRegistration) => {
            // https://developers.google.com/web/updates/2011/12/Transferable-Objects-Lightning-Fast
            registration.active.postMessage(array_buffer, [array_buffer]);
          })
        });
      });
      reader.readAsArrayBuffer(blob);
    }, mime_image);
  }
}

// Canvas setup
canvas.width = canvas.height = size;
context.font = `normal normal normal ${size}px/${size}px sans-serif`;
context.textAlign = 'center';
context.textBaseline = 'middle';

// Link setup
link.rel = 'icon';
link.type = mime_image;
link.setAttribute('sizes', `${size}x${size}`);

// Scan document for statically-defined favicons
const lastlink = [].slice.call(document.getElementsByTagNameNS(ns, 'link'), 0).filter((link: HTMLLinkElement) => {
  return link.rel.toLowerCase() === 'icon' && mime_text_regex.test(link.type);
}).pop();

if (lastlink) {
  const xhr = new XMLHttpRequest;
  const uri = lastlink.href.trim().replace(/^data:(;base64)?,/, "data:text/plain;charset=utf-8$1,");
  xhr.open('GET', uri);
  xhr.addEventListener('load', () => {
    if (xhr.readyState === xhr.DONE && xhr.status === 200) {
      const emoji = xhr.responseText;
      set_emoji_favicon(emoji, false);
    }
  })
  xhr.send();
}
