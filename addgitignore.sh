for dir in *; do
if [ -d $dir ]; then
	echo "$dir"
	git add "$dir"/.gitignore && echo 'added "$dir"/.gitignore'
fi
done 