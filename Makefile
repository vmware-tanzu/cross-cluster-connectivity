.PHONY: addlicense
addlicense:
	# requires https://github.com/google/addlicense
	addlicense -f ./hack/license.txt $(shell find . -path ./hack/tools/vendor -prune -false -o -name *.go)
	addlicense -f ./hack/license.txt $(shell find . -path ./hack/tools/vendor -prune -false -o -name *.sh)
	addlicense -f ./hack/license.txt $(shell find . -path ./hack/tools/vendor -prune -false -o -name Dockerfile)

.PHONY: checklicense
checklicense:
	# requires https://github.com/google/addlicense
	# Ignoring license until there are files in the repo
	# addlicense -check -f ./hack/license.txt $(shell find . -path ./hack/tools/vendor -prune -false -o -name *.go)
	# addlicense -check -f ./hack/license.txt $(shell find . -path ./hack/tools/vendor -prune -false -o -name *.sh)
	# addlicense -check -f ./hack/license.txt $(shell find . -path ./hack/tools/vendor -prune -false -o -name Dockerfile)
