package seeds

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const seedPassword = "Password123!"

// Run inserts deterministic demo data. All inserts use ON CONFLICT DO NOTHING
// so this function is safe to call on every startup.
func Run(ctx context.Context, db *pgxpool.Pool) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(seedPassword), 12)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	h := string(hash)

	steps := []struct {
		name string
		fn   func(context.Context, *pgxpool.Pool, string) error
	}{
		{"locations", seedLocations},
		{"users", func(ctx context.Context, db *pgxpool.Pool, h string) error { return seedUsers(ctx, db, h) }},
		{"members", seedMembers},
		{"coaches", seedCoaches},
		{"suppliers", seedSuppliers},
		{"items", seedItems},
		{"inventory", seedInventory},
		{"classes", seedClasses},
		{"group_buys", seedGroupBuys},
	}

	for _, s := range steps {
		if err := s.fn(ctx, db, h); err != nil {
			return fmt.Errorf("seed %s: %w", s.name, err)
		}
	}
	return nil
}

func seedLocations(ctx context.Context, db *pgxpool.Pool, _ string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO locations (id, name, address) VALUES
		('11111111-0000-0000-0000-000000000001', 'Main Club',         '100 North Michigan Ave, Chicago, IL 60601'),
		('11111111-0000-0000-0000-000000000002', 'Downtown Branch',   '200 West Jackson Blvd, Chicago, IL 60606')
		ON CONFLICT (id) DO NOTHING
	`)
	return err
}

func seedUsers(ctx context.Context, db *pgxpool.Pool, hash string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, first_name, last_name, role) VALUES
		('22222222-0000-0000-0000-000000000001', 'admin@fitcommerce.dev',       $1, 'Admin',        'User',     'administrator'),
		('22222222-0000-0000-0000-000000000002', 'ops@fitcommerce.dev',         $1, 'Operations',   'Manager',  'operations_manager'),
		('22222222-0000-0000-0000-000000000003', 'procurement@fitcommerce.dev', $1, 'Procurement',  'Spec',     'procurement_specialist'),
		('22222222-0000-0000-0000-000000000004', 'coach@fitcommerce.dev',       $1, 'Sarah',        'Coach',    'coach'),
		('22222222-0000-0000-0000-000000000005', 'member@fitcommerce.dev',      $1, 'Mike',         'Member',   'member')
		ON CONFLICT (id) DO NOTHING
	`, hash)
	return err
}

func seedMembers(ctx context.Context, db *pgxpool.Pool, _ string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO members (id, user_id, location_id, membership_type, membership_start, membership_end, status) VALUES
		('33333333-0000-0000-0000-000000000001',
		 '22222222-0000-0000-0000-000000000005',
		 '11111111-0000-0000-0000-000000000001',
		 'standard', '2026-01-01', '2026-12-31', 'active')
		ON CONFLICT (id) DO NOTHING
	`)
	return err
}

func seedCoaches(ctx context.Context, db *pgxpool.Pool, _ string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO coaches (id, user_id, location_id, specialties, bio) VALUES
		('44444444-0000-0000-0000-000000000001',
		 '22222222-0000-0000-0000-000000000004',
		 '11111111-0000-0000-0000-000000000001',
		 ARRAY['HIIT','Yoga','Strength'],
		 'Certified fitness coach with 8 years of experience.')
		ON CONFLICT (id) DO NOTHING
	`)
	return err
}

func seedSuppliers(ctx context.Context, db *pgxpool.Pool, _ string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO suppliers (id, name, contact_name, email, phone) VALUES
		('55555555-0000-0000-0000-000000000001', 'FitGear Wholesale',  'Tom Gear',    'tom@fitgear.example',    '312-555-0101'),
		('55555555-0000-0000-0000-000000000002', 'Supplement Direct',  'Ana Supp',    'ana@supdirect.example',  '312-555-0202'),
		('55555555-0000-0000-0000-000000000003', 'ActiveWear Pro',     'Luis Active', 'luis@activewear.example','312-555-0303')
		ON CONFLICT (id) DO NOTHING
	`)
	return err
}

func seedItems(ctx context.Context, db *pgxpool.Pool, _ string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO items (id, name, description, category, brand, condition, billing_model, deposit_amount, price, status, location_id, created_by) VALUES
		('66666666-0000-0000-0000-000000000001', 'Resistance Band Set',  'Set of 5 resistance bands',        'equipment',    'FitPro',   'new',      'one-time',       50.00,  24.99, 'published', '11111111-0000-0000-0000-000000000001', '22222222-0000-0000-0000-000000000002'),
		('66666666-0000-0000-0000-000000000002', 'Protein Powder 5lb',   'Whey protein, vanilla flavor',     'supplements',  'NutriMax', 'new',      'one-time',       50.00,  54.99, 'published', '11111111-0000-0000-0000-000000000001', '22222222-0000-0000-0000-000000000002'),
		('66666666-0000-0000-0000-000000000003', 'Foam Roller',          'High-density foam roller 36in',    'equipment',    'RollPro',  'new',      'one-time',       50.00,  34.99, 'published', '11111111-0000-0000-0000-000000000001', '22222222-0000-0000-0000-000000000002'),
		('66666666-0000-0000-0000-000000000004', 'Jump Rope Speed',      'Adjustable speed rope',            'equipment',    'SpeedRx',  'new',      'one-time',       50.00,  19.99, 'published', '11111111-0000-0000-0000-000000000001', '22222222-0000-0000-0000-000000000002'),
		('66666666-0000-0000-0000-000000000005', 'Pre-Workout Mix',      'Energy boost, fruit punch',        'supplements',  'BurstFuel','new',      'one-time',       50.00,  39.99, 'published', '11111111-0000-0000-0000-000000000001', '22222222-0000-0000-0000-000000000002'),
		('66666666-0000-0000-0000-000000000006', 'Yoga Mat Premium',     'Non-slip 6mm yoga mat',            'equipment',    'ZenFlex',  'new',      'one-time',       50.00,  29.99, 'published', '11111111-0000-0000-0000-000000000001', '22222222-0000-0000-0000-000000000002'),
		('66666666-0000-0000-0000-000000000007', 'Dumbbells 15lb Pair',  'Rubber hex dumbbells',             'equipment',    'IronEdge','new',       'one-time',       50.00,  49.99, 'published', '11111111-0000-0000-0000-000000000001', '22222222-0000-0000-0000-000000000002'),
		('66666666-0000-0000-0000-000000000008', 'Shaker Bottle 24oz',   'BPA-free shaker with storage',     'accessories',  'MixMate',  'new',      'one-time',       50.00,  14.99, 'published', '11111111-0000-0000-0000-000000000001', '22222222-0000-0000-0000-000000000002'),
		('66666666-0000-0000-0000-000000000009', 'Gym Gloves',           'Full-finger grip gloves',          'accessories',  'GripMax',  'new',      'one-time',       50.00,  22.99, 'published', '11111111-0000-0000-0000-000000000001', '22222222-0000-0000-0000-000000000002'),
		('66666666-0000-0000-0000-000000000010', 'Premium Treadmill',    'Commercial-grade, monthly rental', 'equipment',    'TreadTech','new',      'monthly-rental', 150.00, 199.99,'published', '11111111-0000-0000-0000-000000000001', '22222222-0000-0000-0000-000000000002')
		ON CONFLICT (id) DO NOTHING
	`)
	return err
}

func seedInventory(ctx context.Context, db *pgxpool.Pool, _ string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO inventory_stock (item_id, location_id, on_hand) VALUES
		('66666666-0000-0000-0000-000000000001', '11111111-0000-0000-0000-000000000001', 50),
		('66666666-0000-0000-0000-000000000002', '11111111-0000-0000-0000-000000000001', 30),
		('66666666-0000-0000-0000-000000000003', '11111111-0000-0000-0000-000000000001', 25),
		('66666666-0000-0000-0000-000000000004', '11111111-0000-0000-0000-000000000001', 40),
		('66666666-0000-0000-0000-000000000005', '11111111-0000-0000-0000-000000000001', 60),
		('66666666-0000-0000-0000-000000000006', '11111111-0000-0000-0000-000000000001', 35),
		('66666666-0000-0000-0000-000000000007', '11111111-0000-0000-0000-000000000001', 20),
		('66666666-0000-0000-0000-000000000008', '11111111-0000-0000-0000-000000000001', 80),
		('66666666-0000-0000-0000-000000000009', '11111111-0000-0000-0000-000000000001', 45),
		('66666666-0000-0000-0000-000000000010', '11111111-0000-0000-0000-000000000001',  3)
		ON CONFLICT (item_id, location_id) DO NOTHING
	`)
	return err
}

func seedClasses(ctx context.Context, db *pgxpool.Pool, _ string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO classes (id, coach_id, location_id, name, description, scheduled_at, duration_minutes, capacity, booked_seats, status) VALUES
		('77777777-0000-0000-0000-000000000001',
		 '44444444-0000-0000-0000-000000000001',
		 '11111111-0000-0000-0000-000000000001',
		 'Morning HIIT', 'High-intensity interval training to start the day',
		 NOW() + INTERVAL '1 day', 45, 30, 12, 'scheduled'),

		('77777777-0000-0000-0000-000000000002',
		 '44444444-0000-0000-0000-000000000001',
		 '11111111-0000-0000-0000-000000000001',
		 'Evening Yoga', 'Relaxing flow yoga session',
		 NOW() + INTERVAL '2 days', 60, 20, 8, 'scheduled'),

		('77777777-0000-0000-0000-000000000003',
		 '44444444-0000-0000-0000-000000000001',
		 '11111111-0000-0000-0000-000000000002',
		 'Strength Circuit', 'Full body strength training circuit',
		 NOW() + INTERVAL '3 days', 50, 25, 5, 'scheduled')
		ON CONFLICT (id) DO NOTHING
	`)
	return err
}

func seedGroupBuys(ctx context.Context, db *pgxpool.Pool, _ string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO group_buys (id, item_id, location_id, created_by, title, description, min_quantity, current_quantity, status, cutoff_at, price_per_unit) VALUES
		('88888888-0000-0000-0000-000000000001',
		 '66666666-0000-0000-0000-000000000001',
		 '11111111-0000-0000-0000-000000000001',
		 '22222222-0000-0000-0000-000000000005',
		 'Resistance Band Set — Group Buy',
		 'Join us to unlock a group discount on the FitPro Resistance Band Set. Minimum 10 units needed.',
		 10, 7, 'active',
		 NOW() + INTERVAL '7 days',
		 19.99)
		ON CONFLICT (id) DO NOTHING
	`)
	return err
}
