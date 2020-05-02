require "minitest/autorun"
require "tmpdir"
require "timeout"

def sh(command, expected_exit: 0, timeout: 1)
  result = Timeout.timeout(timeout, RuntimeError, "Timed out when running #{command}") { `#{command} 2>&1` }
  if $?.exitstatus != expected_exit
    raise "Command exited with #{$?.exitstatus}, not the expected #{expected_exit}:\n#{command}\n#{result}"
  end
  result
end

sh "go build .", timeout: 10

describe "logrecycler" do
  def with_config(content)
    Dir.mktmpdir do |dir|
      Dir.chdir(dir) do
        File.write("logrecycler.yaml", content)
        yield
      end
    end
  end

  def call(extra, **args)
    full_path = File.expand_path("./logrecycler", __dir__)
    sh("#{full_path} #{extra}", **args)
  end

  it "can show help" do
    call("--help").must_include "logrecycler"
  end

  it "can show version" do
    call("--version").must_equal "master\n"
  end

  it "fails with unknown arguments" do
    call("--wut", expected_exit: 2).must_include "logrecycler"
  end

  it "fails nicely with no file" do
    with_config "" do
      File.unlink "logrecycler.yaml"
      call("", expected_exit: 2).must_include "open logrecycler.yaml: no such file or directory"
    end
  end

  it "shows location when failing on bad regex in preprocess" do
    with_config "preprocess: '((((WUT'" do
      call("", expected_exit: 1).must_equal "Error: regular expression from preprocess: error parsing regexp: missing closing ): `((((WUT`"
    end
  end

  it "shows location when failing on bad regex in pattern" do
    with_config "patterns:\n- regex: '((((WUT'" do
      call("", expected_exit: 1).must_equal "Error: regular expression from patterns[0].regex: error parsing regexp: missing closing ): `((((WUT`"
    end
  end

  it "fails when using unknown config keys" do
    with_config "oops: 123" do
      call("", expected_exit: 2).must_include "field oops not found"
    end
  end

  it "fails not passing stdin" do
    with_config "" do
      call("", expected_exit: 2).must_include "pipe logs"
    end
  end
end
