/*
 * An image from a message shown large, loaded only when someone opens
 * one. It zooms with the wheel, a pinch or a double click, and Escape or
 * a click outside closes it.
 */
import Lightbox from "yet-another-react-lightbox";
import Zoom from "yet-another-react-lightbox/plugins/zoom";
import "yet-another-react-lightbox/styles.css";

const plugins = [Zoom];

export default function ImageViewer({ src, alt, onClose }: { src: string; alt: string; onClose: () => void }) {
  return (
    <Lightbox
      open
      close={onClose}
      slides={[{ src, alt }]}
      plugins={plugins}
      carousel={{ finite: true }}
      render={{ buttonPrev: () => null, buttonNext: () => null }}
    />
  );
}
