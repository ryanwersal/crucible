# Executed through `brew ruby`, which loads Homebrew's own Ruby and libraries.
# Homebrew has no --no-auto-tap flag. Its deprecated allowlist also exempts
# official taps, so it cannot enforce the no-implicit-taps contract.
require "tap"
require "abstract_command"

module CrucibleNoImplicitTap
  def install(*)
    raise "Crucible refuses to add tap #{name} during a package operation; declare c.brew.tap(#{name.inspect}) and apply again"
  end
end

# Fail closed if Homebrew changes the API this guard depends on.
raise "Unsupported Homebrew: Tap#install is unavailable" unless Tap.method_defined?(:install)
Tap.prepend(CrucibleNoImplicitTap)

module CrucibleHomebrewPackage
  def self.run(operation, arguments)
    raise "Unsupported Homebrew operation" unless %w[install upgrade uninstall].include?(operation)
    require "cmd/#{operation}"
    command = Homebrew::AbstractCommand.command(operation)
    raise "Unsupported Homebrew command API" unless command
    Homebrew.running_command = operation
    Homebrew::API.fetch_api_files! unless Homebrew::EnvConfig.no_install_from_api?
    command.new(arguments).run
    exit(Homebrew.failed? ? 1 : 0)
  end
end
