import Link from "next/link";

export default function NotFound() {
  return (
    <div className="container mx-auto px-4 py-24 max-w-3xl text-center">
      <h1 className="text-4xl font-extrabold tracking-tight text-white">Page not found</h1>
      <p className="mt-4 text-gray-400 leading-relaxed">
        If you followed a download link, visit the Download page to ensure the artifact exists for this release.
      </p>
      <div className="mt-8 flex items-center justify-center gap-4">
        <Link
          href="/"
          className="bg-gray-800 hover:bg-gray-700 text-gray-100 px-5 py-3 rounded-lg text-sm font-semibold transition-colors"
        >
          Home
        </Link>
        <Link
          href="/download"
          className="bg-indigo-600 hover:bg-indigo-500 text-white px-5 py-3 rounded-lg text-sm font-semibold transition-colors"
        >
          Download
        </Link>
      </div>
    </div>
  );
}

