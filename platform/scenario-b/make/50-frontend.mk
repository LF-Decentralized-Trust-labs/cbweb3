include frontend/.env

frontend-spoke-a:
	@./frontend/scripts/spoke-a-stack.sh up

frontend-spoke-b:
	@bash ./frontend/scripts/spoke-b-stack.sh up

frontend-spoke-all:
	@bash ./frontend/scripts/spoke-all-stack.sh up

frontend-scenario-b:
	@bash ./frontend/scripts/scenario-b-stack.sh up

frontend-spoke-a-down:
	@./frontend/scripts/spoke-a-stack.sh down

frontend-spoke-b-down:
	@bash ./frontend/scripts/spoke-b-stack.sh down

frontend-spoke-all-down:
	@bash ./frontend/scripts/spoke-all-stack.sh down

frontend-scenario-b-down:
	@bash ./frontend/scripts/scenario-b-stack.sh down

frontend-spoke-a-logs:
	@./frontend/scripts/spoke-a-stack.sh logs

frontend-spoke-b-logs:
	@bash ./frontend/scripts/spoke-b-stack.sh logs

frontend-spoke-all-logs:
	@bash ./frontend/scripts/spoke-all-stack.sh logs

frontend-scenario-b-logs:
	@bash ./frontend/scripts/scenario-b-stack.sh logs