CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS kalender_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_date DATE NOT NULL,
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    is_booked BOOLEAN DEFAULT FALSE,
    requested_by_name VARCHAR(255),
    requested_by_email VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_kalender_tasks_date ON kalender_tasks(task_date);

INSERT INTO kalender_tasks (task_date, start_time, end_time, title, description, is_booked, requested_by_name, requested_by_email)
VALUES 
  (CURRENT_DATE, '08:00', '10:00', 'Discovery Meeting', 'Project scope and deliverables', false, NULL, NULL),
  (CURRENT_DATE, '10:30', '12:00', 'Architecture Review', 'Schema and query optimization', false, NULL, NULL),
  (CURRENT_DATE, '13:00', '15:00', 'Backend Development', 'API endpoints and middleware', true, NULL, NULL),

  (CURRENT_DATE + INTERVAL '1 day', '09:00', '11:00', 'UI Component Styling', 'Layout and responsive design', false, NULL, NULL),
  (CURRENT_DATE + INTERVAL '1 day', '14:00', '16:00', 'Team Sync', 'Sprint demo and review', true, NULL, NULL),

  (CURRENT_DATE + INTERVAL '3 days', '08:30', '11:30', 'Code Audit', 'Performance tuning', true, 'Alice Smith', 'alice@example.com'),
  (CURRENT_DATE + INTERVAL '3 days', '13:00', '17:00', 'Launch Prep', 'Staging build verification', true, NULL, NULL),

  (CURRENT_DATE + INTERVAL '5 days', '10:00', '12:00', 'Sprint Retrospective', 'Backlog grooming', false, NULL, NULL)
ON CONFLICT DO NOTHING;
