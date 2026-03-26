include frontend/.env

frontend-spoke-a:
	@./frontend/scripts/spoke-a-stack.sh up

frontend-spoke-a-down:
	@./frontend/scripts/spoke-a-stack.sh down

frontend-spoke-a-logs:
	@./frontend/scripts/spoke-a-stack.sh logs