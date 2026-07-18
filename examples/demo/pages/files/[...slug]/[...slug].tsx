// A catch-all page: the folder [...slug] matches one or more trailing
// segments, so it serves /files/a, /files/docs/intro, /files/a/b/c, ...
// useParams() returns the segments as a string[]. This page is
// frontend-only (no .go half) - dynamic pages do not require one.
import { useParams } from "gantry-web";

export default function CatchAll() {
  const { slug } = useParams<{ slug: string[] }>();
  const parts = Array.isArray(slug) ? slug : [slug];
  return (
    <div className="dyn-page">
      <h2>Catch-all route</h2>
      <p>
        The folder <code>pages/files/[...slug]</code> caught {parts.length} segment
        {parts.length === 1 ? "" : "s"}:
      </p>
      <ol className="slug-list">
        {parts.map((p, i) => (
          <li key={i}>{p}</li>
        ))}
      </ol>
    </div>
  );
}
