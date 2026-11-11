import { number } from './dashboardFormat';

type ArticleState = { label: string; value: number; color: string };

export function DashboardContentRing({ states }: { states: ArticleState[] }) {
  const total = states.reduce((sum, { value }) => sum + value, 0);
  let offset = 0;
  const segments = states.map(({ label, value, color }) => {
    const length = total ? (value / total) * 100 : 0;
    const start = offset;
    offset += length;
    return { label, length, start, color };
  });

  return (
    <div className="dashboard-content-overview">
      <div className="dashboard-content-ring">
        <svg viewBox="0 0 144 144" aria-hidden="true" focusable="false">
          <circle cx="72" cy="72" r="60" className="dashboard-ring-track" />
          {segments
            .filter(({ length }) => length > 0)
            .map(({ label, length, start, color }) => {
              const gap = length === 100 ? 0 : Math.min(3.5, length / 2);
              return (
                <circle
                  key={label}
                  cx="72"
                  cy="72"
                  r="60"
                  pathLength="100"
                  fill="none"
                  stroke={color}
                  strokeWidth="8"
                  strokeLinecap={length >= 6 && length < 100 ? 'round' : 'butt'}
                  strokeDasharray={`${length - gap} ${100 - length + gap}`}
                  strokeDashoffset={-(start + gap / 2)}
                />
              );
            })}
        </svg>
        <div className="dashboard-content-total">
          <strong>{number(total)}</strong>
          <span>篇文章</span>
        </div>
      </div>
      <dl className="dashboard-state-list">
        {states.map(({ label, value, color }) => (
          <div key={label}>
            <dt>
              <i style={{ background: color }} aria-hidden="true" />
              {label}
            </dt>
            <dd>{number(value)}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}
