import { useEffect, useState, type FormEvent } from 'react';
import { createIssue, listIssues, type Issue } from './api';
import './App.css';

function App() {
  const [issues, setIssues] = useState<Issue[]>([]);
  const [title, setTitle] = useState('');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    listIssues()
      .then(setIssues)
      .catch((err: unknown) => setError(String(err)));
  }, []);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);

    try {
      const issue = await createIssue({ title });
      setIssues((prev) => [...prev, issue]);
      setTitle('');
    } catch (err: unknown) {
      setError(String(err));
    }
  }

  return (
    <main>
      <h1>IssueTrax</h1>

      <form onSubmit={handleSubmit}>
        <input
          aria-label="Issue title"
          placeholder="New issue title"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
        />
        <button type="submit" disabled={title.trim() === ''}>
          Create
        </button>
      </form>

      {error && <p role="alert">{error}</p>}

      <ul>
        {issues.map((issue) => (
          <li key={issue.id}>
            <span className={`status status-${issue.status}`}>{issue.status}</span> {issue.title}
          </li>
        ))}
      </ul>
    </main>
  );
}

export default App;
