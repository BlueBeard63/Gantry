// A dynamic page: the folder is literally named [id], so it serves
// /examples/page1/1, /examples/page1/2, ... useParams() returns the
// captured segment, and the paired [id].go Model receives it too (TeaView
// renders its View below).
import { useParams, Link } from "gantry-web";
import { TeaView } from "gantry-web/tea";

export default function DynamicId() {
  const { id } = useParams<{ id: string }>();
  return (
    <div className="dyn-page">
      <h2>Dynamic route</h2>
      <p>
        The folder <code>pages/examples/page1/[id]</code> serves every{" "}
        <code>/examples/page1/&lt;id&gt;</code>. This URL's id is <strong>{id}</strong>.
      </p>
      <TeaView />
      <nav className="dyn-nav">
        {["1", "2", "3"].map((n) => (
          <Link key={n} to={`/examples/page1/${n}`} activeClassName="active">
            {n}
          </Link>
        ))}
      </nav>
    </div>
  );
}
