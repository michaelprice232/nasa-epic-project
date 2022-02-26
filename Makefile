deploy_sam_stack:
	./scripts/sam-deploy-aws.sh

delete_sam_stack:
	sam delete --no-prompts


delete_all_items:
	./scripts/clear-bucket-and-database-items.sh