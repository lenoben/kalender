-- Create extension for UUID generation if needed
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Store time-allocated tasks/slots per day
CREATE TABLE IF NOT EXISTS tasks (
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

CREATE INDEX IF NOT EXISTS idx_tasks_date ON tasks(task_date);

-- Insert sample seed data for testing calendar view
INSERT INTO tasks (task_date, start_time, end_time, title, description, is_booked, requested_by_name, requested_by_email)
VALUES 
  (CURRENT_DATE, '08:00', '10:00', 'Client Discovery Meeting', 'Discuss project scope and deliverables', false, NULL, NULL),
  (CURRENT_DATE, '10:30', '12:00', 'System Architecture Review', 'Evaluate Supabase DB schema & indexes', false, NULL, NULL),
  (CURRENT_DATE, '13:00', '15:00', 'Deep Work: Go Backend', 'Implement auth middleware & calendar handler', true, NULL, NULL),

  (CURRENT_DATE + INTERVAL '1 day', '09:00', '11:00', 'UI Component Styling', 'Design glassmorphism modals with Tailwind CSS', false, NULL, NULL),
  (CURRENT_DATE + INTERVAL '1 day', '14:00', '16:00', 'Team Sync & Demo', 'Showcase Go server-rendered calendar grid', true, NULL, NULL),

  (CURRENT_DATE + INTERVAL '3 days', '08:30', '11:30', 'Code Audit & Performance Tuning', 'Optimize pgx query pooling and dynamic indices', true, 'Alice Smith', 'alice@example.com'),
  (CURRENT_DATE + INTERVAL '3 days', '13:00', '17:00', 'Product Launch Prep', 'Final staging build and release verification', true, NULL, NULL),

  (CURRENT_DATE + INTERVAL '5 days', '10:00', '12:00', 'Sprint Retrospective', 'Review accomplishments and backlog grooming', false, NULL, NULL)
ON CONFLICT DO NOTHING;
