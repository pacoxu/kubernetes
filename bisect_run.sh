# found manually the alpha.4 and alpha.3
KNOWN_BAD_COMMIT=v1.28.0-alpha.4
KNOWN_GOOD_COMMIT=v1.28.0-alpha.3

trap cleanup EXIT
cleanup()
{
    echo "reseting..."
    git bisect reset
}

git checkout $KNOWN_BAD_COMMIT
git bisect start
./test.sh
if [ $? -ne 1 ]
then
    echo "error: KNOWN_BAD_COMMIT must be a bad commit!"
    exit 1
fi
git bisect bad

git checkout $KNOWN_GOOD_COMMIT
./test.sh
if [ $? -ne 0 ]
then
    echo echo "error: KNOWN_GOOD_COMMIT= must be a good commit!"
    exit 1
fi
git bisect good

git bisect run ./test.sh
git bisect reset
